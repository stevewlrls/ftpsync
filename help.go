package main

import (
   "github.com/therecipe/qt/widgets"
   "github.com/therecipe/qt/core"
   "github.com/therecipe/qt/gui"
)

/*---------------------------------------------------------------------------
   ViewHelp
      Displays help text in a popup window.
---------------------------------------------------------------------------*/

func ViewHelp () {
   d := widgets.NewQDialog(qMain, 0)
   d.SetWindowTitle("Help - " + gui.QGuiApplication_ApplicationDisplayName())
   
   layout := widgets.NewQVBoxLayout2(d)
   
   text := widgets.NewQTextEdit(nil)
   layout.AddWidget(text, 1, 0)
   text.SetReadOnly(true)
	
	hf := core.NewQFile2(":/help.html")
	help := helpText
	if hf.Open(core.QIODevice__ReadOnly | core.QIODevice__Text) {
		help = hf.ReadAll().Data()
		hf.Close()
	}
	text.SetHtml(help)
   
   buttons := widgets.NewQDialogButtonBox3(
      widgets.QDialogButtonBox__Ok,
      nil,
   )
   layout.AddWidget(buttons, 1, 0)
   
   buttons.ConnectAccepted(d.Accept)
   buttons.ConnectRejected(d.Reject)
   
   d.Exec()
}

/* Help text for popup
**    This uses a simplified HTML format, as supported by the Qt 'TextEdit'
** widget. Note that the full help text is loaded from resource file
** "help.html"; this is just a place-holder in case that file cannot be
** opened.
*/

var helpText = "<p>The <b>ftpsync</b> program compares files in a given source directory (and sub-directories) with a supposedly equivalent set on a remote server. These files may, for example, comprise a web site. Any differences are shown in the resulting 'report'. Unlike a plain FTP application, <b>ftpsync</b> compares the file contents, creating a MD5 'fingerprint' for each one. If the local and remote fingerprints match - even if size and modification times are slightly different - the files are assumed to be identical.</p>"