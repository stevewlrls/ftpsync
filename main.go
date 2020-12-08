/*
** Module:  ftpsync
** Package: main
** Version: 1.0
** Author:  Steven Wheeler
** URL:     http://www.mirandesign.co.uk
**
** 'ftpsync' is a stand-alone program to compare a local folder with one on a
** remote server and report the 'real' differences. It has been written because
** a number of web/ftp servers and/or clients do not synchronise the "last
** modified" date and time - either at all, or correctly. Furthermore, the
** different ways that Linux and Windows store text files means that file sizes
** may also be slightly different.
**
** This program scans the local and remote folders and constructs a "fingerprint"
** for each file, comprising its local "last modified" timestamp, its size in
** bytes and a "digest" (MD5) of the contents. The latter is recomputed when
** either of the first two change. The local and remote copies are then tested
** for 'equality' by the value of the digest. There is a small chance that the
** files could be different but have the same digest, rising if a malicious user
** has deliberately tampered with the file to make its digest the same.
**
** Differences in text file format are handled by fetching the remote file in
** ASCII mode. We hope that the FTP server is not stripping the top bit, else
** UTF-8 characters could get "munged". The result of using ACII mode should be
** that the local and fetched copy use the same text format, regardless of what
** the remote server uses.
*/

package main

import (
   "log"
   "fmt"
   "os"
   "github.com/therecipe/qt/widgets"
   "github.com/therecipe/qt/gui"
)

var qMain *MainWindow

/*---------------------------------------------------------------------------
   main
      Program entrypoint. (Self-explanatory.)
---------------------------------------------------------------------------*/

func main () {
   app := widgets.NewQApplication(len(os.Args), os.Args)
	app.ConnectAboutToQuit(func () {
		qMain.EndScan()
		Config.Save()
	})
   ParseOptions()
   qMain = NewMainWindow(nil, 0)
   qMain.Show()
   app.Exec()
}

/*---------------------------------------------------------------------------
                                 MainWindow
---------------------------------------------------------------------------*/

type MainWindow struct {
   widgets.QMainWindow
   
   report      *ReportView
   cache       *Cache
   selected    *FilePrint
   siteMenu    *widgets.QMenu
	
	errors		chan error
	abort			chan bool
	scanState	int
   
   _ func() `constructor:"init"`
	_ func(string) `signal:"ShowStatus"`
	_ func() `signal:"ScanComplete"`
}

const (
	Scanner__Idle = iota
	Scanner__Active
	Scanner__Stopping
)

/* init
**    Callback to construct the user interface components within the
** application main window.
*/

func (w *MainWindow) init () {
   w.report = NewReportView(w)
   w.SetCentralWidget(w.report)
   
   menuBar := w.MenuBar()
   tools := w.AddToolBar3("Tools")
   tools.SetFloatable(false)
   tools.SetMovable(false)
   
   menu := menuBar.AddMenu2("&File")
   act := menu.AddAction2(gui.NewQIcon5(":/images/settings.png"), "Settings")
   tools.InsertAction(nil, act)
   act.ConnectTriggered(w.configure)
   
   act = menu.AddAction("Quit")
   act.ConnectTriggered(func (bool) { w.Close() })
   
   w.siteMenu = menuBar.AddMenu2("Si&te")
   w.buildSiteMenu()
   
   menu = menuBar.AddMenu2("&Scan")
   act = menu.AddAction2(gui.NewQIcon5(":/images/search.png"), "Begin Scan")
   tools.InsertAction(nil, act)
   act.ConnectTriggered(w.beginScan)
   
   act = menu.AddAction2(gui.NewQIcon5(":/images/reset.png"), "Reset")
   tools.InsertAction(nil, act)
   act.ConnectTriggered(w.resetScan)
   
   menu = menuBar.AddMenu2("&Report")
   doView := menu.AddAction2(gui.NewQIcon5(":/images/preview.png"), "View Changes")
   tools.InsertAction(nil, doView)
   doView.ConnectTriggered(w.viewSelected)
   doView.SetEnabled(false)
   
   menu = menuBar.AddMenu2("&Help")
   act = menu.AddAction("About")
   act.ConnectTriggered(w.showVersion)
   
   act = menu.AddAction("Usage")
   act.ConnectTriggered(w.showUsage)
   
   w.report.ConnectRowSelected(func (yes bool) { doView.SetEnabled(yes) })
	
	w.ConnectShowStatus(w.showStatus)
	w.ConnectScanComplete(w.scanComplete)
	w.errors = make(chan error, 1)
	w.abort = make(chan bool, 1)
	w.scanState = Scanner__Idle
   
   w.StatusBar()
   w.SetWindowIcon(gui.NewQIcon5(":/images/search.png"))
   
   w.refresh()
}

/* buildSiteMenu
**    Builds or rebuilds the site selection menu. Called on startup and after
** using the site manager config dialog.
*/

func (w *MainWindow) buildSiteMenu () {
   w.siteMenu.Clear()
   
   for _, s := range Sites.List {
      act := w.siteMenu.AddAction(s.Name)
      act.ConnectTriggered(s.MakeCurrent)
   }
   
   if len(Sites.List) > 0 { w.siteMenu.AddSeparator() }
   
   act := w.siteMenu.AddAction("New")
   act.ConnectTriggered(w.configure)
}

/* configure
**    Shows a property dialog to configure scan options.
*/

func (w *MainWindow) configure (bool) {
   d := NewConfigDialog(w, 0)
   d.Exec()
   Config.MakeCurrent(true)
}

/* refresh
**    Updates the main window title to reflect the currently selected site and
** re-loads the results of the last scan (if any).
*/

func (w *MainWindow) refresh () {
	title := gui.QGuiApplication_ApplicationDisplayName()
	if Config.Source != "" { title = fmt.Sprintf("%s - %s", Config.Name, title) }
	w.SetWindowTitle(title)
   
   if Config.Source == "" {
      w.cache = NewCache()
      w.report.SetModel(nil)
   } else {
      var err error
      w.cache, err = LoadCache()
      if err != nil { w.showError("Load cache", err); return }
      w.report.SetModel(ShowResults(w.cache))
   }
}

/* beginScan
**    Handles the menu item to begin a new scan.
*/

func (w *MainWindow) beginScan (bool) {
	if w.scanState != Scanner__Idle { return }
   err := Config.Check()
   if err == nil {
      if Opt.Verbose {
         log.Println("Starting scan...")
         log.Printf("  Local folder: %s\n", Config.Source)
         log.Printf("  Cache file:   %s\n", Config.CacheFile)
         log.Printf("  Exclude:      %v\n", Config.Exclude)
         log.Printf("  Binary:       %v\n", Config.BinaryFiles)
         log.Printf("  Remote addr:  %s\n", Config.RemoteAddr.String())
         log.Printf("  Server key:   %x\n", Config.ServerKey)
      }
		w.scanState = Scanner__Active
      go ScanFolders(w.cache, w.errors, w.abort)
   } else {
		w.showError("Scan", err)
   }
}

/* scanComplete
**		Signal received when scan goroutine finishes. The status must be sent on
** the 'errors' channel, as it contains a Go error value, which cannot be sent
** via Qt.
*/

func (w *MainWindow) scanComplete () {
	if w.scanState != Scanner__Idle {
		err := <- w.errors
		if err != nil {
			w.TempStatus("Scan failed")
			w.showError("Scan", err)
		} else {
			w.TempStatus("Scan complete")
			w.cache.Write()
			w.report.SetModel(ShowResults(w.cache))
		}
	}
	w.scanState = Scanner__Idle
}

/* EndScan
**		Called to abort the current scan on program exit. Because the Qt event
**	loop has already finished at this point, we call 'scanComplete' manually,
** to wait for the signal from the scanner GoRoutine that it has finished.
*/

func (w *MainWindow) EndScan () {
	if w.scanState == Scanner__Active {
		w.scanState = Scanner__Stopping
		w.abort <- true
		err := <- w.errors
		if err != nil { log.Println("Scan:", err) } else {
			w.cache.Write()
		}
	}
}

/* resetScan
**    If confirmed by the user, deletes the cache file for the current site.
** This will force a full re-scan on the next run.
*/

func (w *MainWindow) resetScan (bool) {
	if w.scanState != Scanner__Idle { return }
   answer := widgets.QMessageBox_Question(
      w,
      "Reset Scan",
      "Do you really want to clear cached results for this site?",
      widgets.QMessageBox__Ok | widgets.QMessageBox__Cancel,
      widgets.QMessageBox__NoButton,
   )
   
   if answer == widgets.QMessageBox__Ok {
      w.cache = NewCache()
      w.report.SetModel(nil)
      w.cache.Write()
   }
}

/* viewSelected
**    Shows the differences between local & remote copies of the currently
** selected file.
*/

func (w *MainWindow) viewSelected (bool) {
   err := w.report.ViewSelected()
   if err != nil { w.showError("View", err) }
}

/* showError
**    Shows an error message.
*/

func (w *MainWindow) showError (scope string, err error) {
   widgets.QMessageBox_Critical(
      w,
      "Error",
      fmt.Sprintf("%s: %v", scope, err),
      widgets.QMessageBox__Ok,
      widgets.QMessageBox__NoButton,
   )
}

/* showStatus
**    Displays a message in the main window status bar. The message is
** temporary, so the next message will displace it, but will not disappear
** until replaced.
*/

func (w *MainWindow) showStatus (msg string) {
   w.StatusBar().ShowMessage(msg, 0)
}


/* TempStatus
**    Displays a message in the main window status bar with 3 second
** timeout.
*/

func (w *MainWindow) TempStatus (msg string) {
   w.StatusBar().ShowMessage(msg, 3000)
}

/* showVersion
**    Displays a brief dialog to giev the current software version.
*/

func (w *MainWindow) showVersion (bool) {
   d := widgets.NewQMessageBox2(
      widgets.QMessageBox__NoIcon,
      "About",
      "Version 1.0\nDecember 2020",
      widgets.QMessageBox__Ok,
      w, 0,
   )
   d.SetIconPixmap(gui.NewQPixmap3(":/images/search.png", "", 0))
   d.SetInformativeText("Copyright © 2010, Miran Design")
   d.SetWindowTitle("About " + gui.QGuiApplication_ApplicationDisplayName())
   d.Exec()
}

/* showUsage
**    Displayes scrollable help on application usage.
*/

func (w *MainWindow) showUsage (bool) {
   ViewHelp()
}