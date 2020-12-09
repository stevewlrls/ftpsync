package app

import (
   "io/ioutil"
   "strings"
   "html"
   "path/filepath"
   dmp "github.com/sergi/go-diff/diffmatchpatch"
   "github.com/therecipe/qt/widgets"
   "github.com/therecipe/qt/core"
   "github.com/therecipe/qt/gui"
)

/*---------------------------------------------------------------------------
   ViewFile
      Fetches the remote and local copies of a given file, identified by
   relative path, and compares them to detect differences. Then displays
   the annotated text in a popup window.
---------------------------------------------------------------------------*/

func ViewFile (path string) error {
   qMain.TempStatus("Fetching remote copy ...")
   
   conn, err := DialRemote()
   if err != nil { return err }
   defer conn.Close()
   
   var buf strings.Builder
   err = conn.Retrieve(filepath.Join(Config.RemoteAddr.Path, path), &buf)
   if err != nil { return err }
   
   text1, err := ioutil.ReadFile(filepath.Join(Config.Source, path))
   if err != nil { return err }
   
   d := dmp.New()
   diffs := d.DiffMain(string(text1), buf.String(), false)
   d.DiffCleanupSemantic(diffs)
   
   // Load difference text and show dialog
   
   if viewer == nil { viewer = NewFileViewer(nil, 0) }
   viewer.SetPath(path)
   viewer.SetHtml(diffs)
   viewer.Open()
   
   return nil
}

/*---------------------------------------------------------------------------
   FileViewer [type]
      Popup window to show differences between local and remote copies of
   a selected file.
---------------------------------------------------------------------------*/

type FileViewer struct {
   widgets.QDialog
   
   path     *widgets.QLabel
   text     *widgets.QTextEdit
   
   _ func() `constructor:"init"`
}

var viewer *FileViewer

/* init
**    Creates child widgets to display rich text.
*/

func (d *FileViewer) init () {
   layout := widgets.NewQVBoxLayout2(d)
   
   d.path = widgets.NewQLabel(nil, 0)
   layout.AddWidget(d.path, 0, 0)
   
   d.text = widgets.NewQTextEdit(nil)
   layout.AddWidget(d.text, 1, 0)
   //d.text.SetLineWrapMode(widgets.QTextEdit__NoWrap)
   d.text.SetReadOnly(true)
   
   info := widgets.NewQLabel(nil, 0)
   info.SetTextFormat(core.Qt__RichText)
   info.SetText("Key: <ins style=\"color:#329ea8;\">New in remote;</ins> unchanged; <del style=\"background-color:#c2c2c2;color:#b51919;text-decoration:line-through;\">new in local.</del>")
   layout.AddWidget(info, 0, 0)
   
   buttons := widgets.NewQDialogButtonBox3(
      widgets.QDialogButtonBox__Ok,
      nil,
   )
   layout.AddWidget(buttons, 1, 0)
   
   buttons.ConnectAccepted(d.Accept)
   buttons.ConnectRejected(d.Reject)
}

/* SetPath
**    Sets the path string for the file being viewed. This is used in a label
** above the text and (as basename only) in the window title.
*/

func (d *FileViewer) SetPath (path string) {
   d.SetWindowTitle(filepath.Base(path) + " - " + gui.QGuiApplication_ApplicationDisplayName())
   d.path.SetText("Viewing " + path)
}

/* SetHtml
**    Translates the differences between local and remote copies into a
** rich text (HTML) representation that highlights insertions and deletions.
*/

func (d *FileViewer) SetHtml (diffs []dmp.Diff) {
   var buf strings.Builder
	for _, d := range diffs {
		text := strings.ReplaceAll(html.EscapeString(d.Text), "\n", "<br>")
		switch d.Type {
		case dmp.DiffInsert:
			buf.WriteString("<ins style=\"color:#329ea8;\">")
			buf.WriteString(text)
			buf.WriteString("</ins>")
		case dmp.DiffDelete:
         buf.WriteString("<del style=\"background-color:#c2c2c2;color:#b51919;text-decoration:line-through;\">")
			buf.WriteString(text)
			buf.WriteString("</del>")
		case dmp.DiffEqual:
			buf.WriteString("<span>")
			buf.WriteString(text)
			buf.WriteString("</span>")
		}
	}
   d.text.SetHtml(buf.String())
}