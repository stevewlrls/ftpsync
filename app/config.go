package app

/*
** The routines in this file manage the list of known "sites", keeping for
** each details of the local and remote paths and server connection keys.
*/

import (
   "os"
   "log"
   "strings"
   "errors"
   "fmt"
   "path/filepath"
   "net/url"
   "encoding/gob"
   "github.com/therecipe/qt/core"
   "github.com/therecipe/qt/widgets"
)

/*---------------------------------------------------------------------------
   Global variables
---------------------------------------------------------------------------*/

var Config *SiteConfig

var Sites struct {
   List        []*SiteConfig
   Current     int
}

/*---------------------------------------------------------------------------
   init
      Called on startup. Loads the site list from a standard location.
---------------------------------------------------------------------------*/

func init () {
   path, err := os.UserConfigDir();
   if err != nil { log.Fatalf("Cannot determine location for saved config (%v)", err) }
   
   path = filepath.Join(path, "ftpsync")
   os.MkdirAll(path, 0700)
   
   f, err := os.Open(filepath.Join(path, "ftpsync.conf"))
   if err == nil {
      defer f.Close()
      dec := gob.NewDecoder(f)
      err = dec.Decode(&Sites)
      if err != nil { log.Fatalf("Bad format for saved config. (%v)", err) }
   } else {
      if os.IsNotExist(err) {
         Sites.List = make([]*SiteConfig, 0)
         Sites.Current = -1
      }
   }
   
   if Sites.Current < 0 {
      Config = Config.New()
   } else { Config = Sites.List[Sites.Current] }
}

/*---------------------------------------------------------------------------
   SiteConfig [type]
---------------------------------------------------------------------------*/

type SiteConfig struct {
   Name,
   Source,
   CacheFile,
   Exclude,
   BinaryFiles string
   RemoteAddr  *url.URL
   ServerKey   []byte
   
   // session-only (not saved)
   password    string
}

/* New
**    Adds a new (empty) site to the list.
*/

func (_ *SiteConfig) New () (site *SiteConfig) {
   remote, _ := url.Parse("ftps://")
   site = &SiteConfig{
      Name:          "New site",
      CacheFile:     ".ftpsync-cache",
      RemoteAddr:    remote,
      Exclude:       "@.gitignore|.DS_Store|_vti_cnf|_vti_pvt|thumbs.db|.git",
      BinaryFiles:   ".jar|.phar|.zip|.mp3|.mp4|.ogg|.mkv|.png|.gif|.jpg|.jpeg",
   }
   return
}

/* Check
**    Checks whether the current site config is valid. If not, returns a
** suitable descriptive error.
*/

func (c *SiteConfig) Check () error {
   if c.Source == "" {
      return errors.New("No source configured for scan") 
   }
   if c.RemoteAddr.Host == "" {
      return errors.New("No remote server configured for scan")
   }
   if c.CacheFile == "" {
      return errors.New("Cache file name must not be blank")
   }
   return nil
}

/* Save
**    Called as the program is about to exit and after making edits. Saves
** modified site data to disk.
*/

func (_ *SiteConfig) Save () {
   path, _ := os.UserConfigDir();
   f, err := os.Create(filepath.Join(path, "ftpsync/ftpsync.conf"))
   if err == nil {
      defer f.Close()
      enc := gob.NewEncoder(f)
      err = enc.Encode(&Sites)
   }
   
   if err != nil { log.Printf("Error saving site config. (%v)", err) }
}

/* MakeCurrent
**    Makes the given site "current".
*/

func (s *SiteConfig) MakeCurrent (_ bool) {
   Sites.Current = -1
   
   for n, v := range Sites.List {
      if v == s {
         Sites.Current = n
         Config = v
         break
      }
   }
   
   if Sites.Current < 0 { Config = Config.New() }
   
   Config.Save()
   qMain.refresh()
}

/* Select
**    Searches for a given site name (specified on the command line) and
** makes it 'current'.
*/

func (_ *SiteConfig) Select (name string) error {
   for _, v := range Sites.List {
      if strings.EqualFold(name, v.Name) {
         v.MakeCurrent(true)
         return nil
      }
   }
   return fmt.Errorf("No such site: %s", name)
}

/*---------------------------------------------------------------------------
   ConfigDialog [widget]
---------------------------------------------------------------------------*/

type ConfigDialog struct {
   widgets.QDialog
   
   detail      *SiteDetailPane
   model       *SiteListModel
   
   _ func() `constructor:"init"`
}

/* init
**    Creates the widgets that form the 'site manager' popup.
*/

func (d *ConfigDialog) init () {
   layout := widgets.NewQGridLayout(d)
   
   layout.AddWidget3(widgets.NewQLabel2("Sites", nil, 0), 0, 0, 1, 1, 0)
   
   list := widgets.NewQListView(nil)
   layout.AddWidget3(list, 1, 0, 1, 1, 0)
   d.model = NewSiteListModel(nil); list.SetModel(d.model)
   if Sites.Current > 0 { list.SetCurrentIndex(d.model.Current()) }
   
   row := widgets.NewQHBoxLayout()
   row.SetContentsMargins(0, 0, 0, 0)
   bnAdd := widgets.NewQPushButton2("+", nil); row.AddWidget(bnAdd, 0, 0)
   bnDrop := widgets.NewQPushButton2("-", nil); row.AddWidget(bnDrop, 0, 0)
   row.AddStretch(1)
   layout.AddLayout2(row, 2, 0, 1, 1, 0)
   
   buttons := widgets.NewQDialogButtonBox(nil)
   layout.AddWidget3(buttons, 3, 0, 1, 2, 0)
   buttons.SetStandardButtons(
      widgets.QDialogButtonBox__Ok,
   )
   
   d.detail = NewSiteDetailPane(nil)
   layout.AddLayout2(d.detail, 0, 1, 3, 1, 1)
   
   layout.SetColumnStretch(1, 2)
   
   // Connect actions ...
   
   list.ConnectActivated(d.showDetail)
   
   buttons.ConnectAccepted(d.Accept)
   buttons.ConnectRejected(d.Reject)
   
   bnAdd.ConnectClicked(func (bool) {
      index := d.model.AddRow()
      list.SetCurrentIndex(index)
      d.showDetail(index)
   })
   
   bnDrop.ConnectClicked(func (bool) {
      ans := widgets.QMessageBox_Question(
         nil,
         "Delete Site",
         "Are you sure you want to delete site " + Config.Name + "?",
         widgets.QMessageBox__Yes | widgets.QMessageBox__No,
         widgets.QMessageBox__NoButton,
      )
      if ans == widgets.QMessageBox__Yes {
         index := d.model.DeleteRow()
			if index.IsValid() { list.SetCurrentIndex(index) }
         d.showDetail(index)
      }
   })
   
   d.detail.ConnectEdited(func () { d.model.Updated() })
   
   // Select current entry (if any)
   
   if Sites.Current >= 0 {
      list.SetCurrentIndex(d.model.Current())
      d.detail.ShowSite()
   }
}

/* showDetail
**    Shows the detail for a site when that row is selected in the list.
*/

func (d *ConfigDialog) showDetail (index *core.QModelIndex) {
   if index.IsValid() {
		Sites.Current = index.Row()
		Config = Sites.List[Sites.Current]
		d.detail.ShowSite()
	} else {
		d.detail.Reset()
	}
}

/*---------------------------------------------------------------------------
   SiteDetailPane [layout]
---------------------------------------------------------------------------*/

type SiteDetailPane struct {
   widgets.QFormLayout
   
   name,
   server,
   remotePath,
   username,
   password,
   cacheFile,
   exclude,
   binary      *widgets.QLineEdit
   source      *FileSelector
   scheme      *widgets.QComboBox
	advanced		*widgets.QPushButton
	frame			*widgets.QGroupBox
   
   _ func() `constructor:"init"`
   _ func() `signal:"Edited"`
}

/* init
**    Adds the widgets that show and allow editing of the site detail fields.
*/

func (p *SiteDetailPane) init () {
   p.SetContentsMargins(0, 0, 0, 0)
   p.SetFieldGrowthPolicy(widgets.QFormLayout__ExpandingFieldsGrow)
   p.AddRow5(widgets.NewQLabel2("Site detail:", nil, 0))
   
   p.name = widgets.NewQLineEdit(nil); p.AddRow3("Name", p.name)
   p.source = NewFileSelector(nil, 0); p.AddRow3("Source", p.source)
   p.server = widgets.NewQLineEdit(nil); p.AddRow3("Server", p.server)
   p.scheme = widgets.NewQComboBox(nil); p.AddRow3("Scheme", p.scheme)
   p.scheme.AddItems([]string{"FTP (insecure)", "FTPS with explicit TLS", "SFTP"})
   p.remotePath = widgets.NewQLineEdit(nil); p.AddRow3("Root folder", p.remotePath)
   p.username = widgets.NewQLineEdit(nil); p.AddRow3("Username", p.username)
   p.password = widgets.NewQLineEdit(nil); p.AddRow3("Password", p.password)
   p.password.SetEchoMode(widgets.QLineEdit__Password)
   
   p.advanced = widgets.NewQPushButton2("Advanced", nil); p.AddRow3("", p.advanced)
   
   p.frame = widgets.NewQGroupBox2("Advanced Options", nil)
   p.AddRow5(p.frame); p.frame.Hide()
   opt := widgets.NewQFormLayout(p.frame)
   opt.SetFieldGrowthPolicy(widgets.QFormLayout__ExpandingFieldsGrow)
   
   p.cacheFile = widgets.NewQLineEdit(nil); opt.AddRow3("Cache file", p.cacheFile)
   p.exclude = widgets.NewQLineEdit(nil); opt.AddRow3("Exclude", p.exclude)
   p.binary = widgets.NewQLineEdit(nil); opt.AddRow3("Binary", p.binary)
   
   // Connect actions ...
   
   p.name.ConnectTextEdited(func (text string) { Config.Name = text; p.Edited() })
   p.source.ConnectPathChanged(func (text string) { Config.Source = text })
   p.server.ConnectTextEdited(func (text string) {
      Config.RemoteAddr.Host = text; Config.ServerKey = nil
   })
   p.scheme.ConnectCurrentIndexChanged(func (n int) {
		if n >= 0 { Config.RemoteAddr.Scheme = []string{"ftp", "ftps", "sftp"}[n] }
   })
   p.remotePath.ConnectTextEdited(func (text string) { Config.RemoteAddr.Path = text })
   p.username.ConnectTextEdited(p.setUser)
   p.password.ConnectTextEdited(p.setUser)
   p.cacheFile.ConnectTextEdited(func (text string) { Config.CacheFile = text })
   p.exclude.ConnectTextEdited(func (text string) { Config.Exclude = text })
   p.binary.ConnectTextEdited(func (text string) { Config.BinaryFiles = text })
   
   p.advanced.ConnectClicked(func (bool) { p.advanced.Hide(); p.frame.Show() })
}

/* ShowSite
**    Displays the details for the currently selected site.
*/

func (p *SiteDetailPane) ShowSite () {
   p.name.SetText(Config.Name)
   p.source.SetText(Config.Source)
   p.server.SetText(Config.RemoteAddr.Host)
   for n, v := range []string{"ftp", "ftps", "sftp"} {
      if Config.RemoteAddr.Scheme == v {
         p.scheme.SetCurrentIndex(n); break
      }
   }
   p.remotePath.SetText(Config.RemoteAddr.Path)
   p.username.SetText(Config.RemoteAddr.User.Username())
   pwd, _ := Config.RemoteAddr.User.Password()
   p.password.SetText(pwd)
   p.cacheFile.SetText(Config.CacheFile)
   p.exclude.SetText(Config.Exclude)
   p.binary.SetText(Config.BinaryFiles)
	p.frame.Hide()
	p.advanced.Show()
}

/* Reset
**		Resets the detail pane to initial state.
*/

func (p *SiteDetailPane) Reset () {
	p.name.Clear()
	p.source.Clear()
	p.server.Clear()
	p.scheme.Clear()
	p.remotePath.Clear()
	p.username.Clear()
	p.password.Clear()
	p.cacheFile.Clear()
	p.exclude.Clear()
	p.binary.Clear()
	p.frame.Hide()
	p.advanced.Show()
}

/* setUser
*/

func (p *SiteDetailPane) setUser (string) {
   user := p.username.Text()
   pwd := p.password.Text()
   if pwd == "" { Config.RemoteAddr.User = url.User(user) } else {
      Config.RemoteAddr.User = url.UserPassword(user, pwd)
   }
}

/*---------------------------------------------------------------------------
   FileSelector [widget]
---------------------------------------------------------------------------*/

type FileSelector struct {
   widgets.QWidget
   
   input       *widgets.QLineEdit
   
   _ func() `constructor:"init"`
   _ func(string) `signal:"PathChanged"`
}

/* init
**    Adds the input field and 'browse' button.
*/

func (w *FileSelector) init () {
   layout := widgets.NewQHBoxLayout2(w)
   layout.SetContentsMargins(0, 0, 0, 0)
   
   w.input = widgets.NewQLineEdit(nil); layout.AddWidget(w.input, 1, 0)
   w.input.SetSizePolicy2(
      widgets.QSizePolicy__MinimumExpanding,
      widgets.QSizePolicy__Fixed,
   )
   browse := widgets.NewQPushButton2("Browse", nil)
   layout.AddWidget(browse, 0, 0)
   
   // Connect actions ...
   
   browse.ConnectClicked(w.browse)
   w.input.ConnectTextChanged(w.PathChanged)
}

/* browse
**    Handles "press" event on the browse button. Opens a file selection
** dialog to choose a local file and stores the resulting path in the input
** field.
*/

func (w *FileSelector) browse (bool) {
   dir := widgets.QFileDialog_GetExistingDirectory(
      nil,
      "Source Folder",
      w.input.Text(),
      0,
   )
   w.input.SetText(dir)
}

/* SetText
**    Sets the text for the line-edit part of this composite widget.
*/

func (w *FileSelector) SetText (text string) {
   w.input.SetText(text)
}

/* Clear
**		Resets the file path.
*/

func (w *FileSelector) Clear () {
	w.input.Clear()
}

/*---------------------------------------------------------------------------
   SiteListModel [model]
---------------------------------------------------------------------------*/

type SiteListModel struct {
   core.QAbstractListModel
   
   _ func() `constructor:"init"`
}

/* init
**    Sets up the abstract model to return and update the global site list.
*/

func (m *SiteListModel) init () {
   m.ConnectRowCount(func (*core.QModelIndex) int { return len(Sites.List) })
   m.ConnectData(func (index *core.QModelIndex, role int) *core.QVariant {
      if index.IsValid() {
         site := Sites.List[index.Row()]
         switch core.Qt__ItemDataRole(role) {
            case core.Qt__DisplayRole:
               return core.NewQVariant1(site.Name)
            case core.Qt__ToolTipRole:
               return core.NewQVariant1(site.Source)
            case core.Qt__WhatsThisRole:
               return core.NewQVariant1("Site name")
         }
      }
      return core.NewQVariant()
   })
}

/* Current
**    Returns a model index for the current active site.
*/

func (m *SiteListModel) Current () *core.QModelIndex {
   return m.CreateIndex(Sites.Current, 0, nil)
}

/* AddRow
**    Adds a new (empty) site to the list.
*/

func (m *SiteListModel) AddRow () *core.QModelIndex {
   root := core.NewQModelIndex()
   m.BeginInsertRows(root, len(Sites.List), len(Sites.List))
   Sites.List = append(Sites.List, Config.New())
   m.EndInsertRows()
   return m.CreateIndex(len(Sites.List) - 1, 0, nil)
}

/* DeleteRow
**    Removes the currently selected site from the list and returns a model
** index for the one to make the new current site (if any).
*/

func (m *SiteListModel) DeleteRow () *core.QModelIndex {
   root := core.NewQModelIndex()
   if Sites.Current < 0 { return root }
   
   m.BeginRemoveRows(root, Sites.Current, Sites.Current)
   if Sites.Current < len(Sites.List) - 1 {
      Sites.List = append(Sites.List[:Sites.Current], Sites.List[Sites.Current+1:]...)
   } else {
      Sites.List = Sites.List[:Sites.Current]
   }
   m.EndRemoveRows()
   
   if Sites.Current >= len(Sites.List) {
      Sites.Current--
      if Sites.Current < 0 { Config = Config.New(); return root }
   }
   
   return m.Current()
}

/* Updated
**    Called when the current site detail has been changed. Emits the right
** signal to cause the view to update.
*/

func (m *SiteListModel) Updated () {
   index := m.Current()
   m.DataChanged(index, index, nil)
}