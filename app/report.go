package app

/*
** This file contains the logic to generate a report of the mis-matches between
** local and remote copies.
*/

import (
   "github.com/therecipe/qt/widgets"
   "github.com/therecipe/qt/core"
   "github.com/therecipe/qt/gui"
)

var ReportIcons map[string]*gui.QIcon

/*---------------------------------------------------------------------------
   init
      Module initialisation. Creates the icon cache.
---------------------------------------------------------------------------*/

func init () {
   ReportIcons = make(map[string]*gui.QIcon)
}

/*---------------------------------------------------------------------------
   ShowResults
      Builds up a report as a Qt table model.
---------------------------------------------------------------------------*/

func ShowResults (cache *Cache) (model *ResultsModel) {
   model = NewResultsModel(nil)
   
   if len(ReportIcons) == 0 {
      for _, ic := range []string{"upload", "download", "conflict"} {
         ReportIcons[ic] = gui.NewQIcon5(":/images/" + ic + ".png")
      }
   }
   
   cache.Walk(func (path string, fp *FilePrint) {
      if path == "." { return }
      if ! (fp.Local.Changed || fp.Remote.Changed) { return }
      
      var (lc, rc *gui.QIcon; ls, rs string)
      if fp.Local.Changed {
         lc = ReportIcons["upload"]
         if fp.Remote.Changed {
            if fp.Remote.ModTime.IsZero() { rs = "X" } else { lc = ReportIcons["conflict"]; rc = lc }
         } else { rs = "-" }
      } else {
         ls = "-"
         if fp.Remote.Changed { rc = ReportIcons["download"] } else { rs = ls }
      }
      
      model.AppendRow([]*gui.QStandardItem{
         gui.NewQStandardItem2(path),
         gui.NewQStandardItem3(lc, ls),
         gui.NewQStandardItem3(rc, rs),
      })
   })
   
   return
}

/*---------------------------------------------------------------------------
   ResultsModel [type]
---------------------------------------------------------------------------*/

type ResultsModel struct {
   gui.QStandardItemModel
   
   _ func() `constructor:"init"`
}

/* init
**    Sets up model headings, etc..
*/

func (m *ResultsModel) init () {
   m.SetHorizontalHeaderLabels([]string{"Path", "Local", "Remote"})
}

/*---------------------------------------------------------------------------
   ReportView [widget]
---------------------------------------------------------------------------*/

type ReportView struct {
   widgets.QTableView
   
   _ func() `constructor:"init"`
   _ func(bool) `signal:"rowSelected"`
}

/* init
**    Sets up an empty model for the initial report view and adds a custom
** item delegate to center the 'local' and 'remote' columns.
*/

func (w *ReportView) init () {
   w.SetModel(nil)
   w.SetSelectionBehavior(widgets.QAbstractItemView__SelectRows)
   w.SetMinimumWidth(450)
   
   d := NewCenteredItemDelegate(nil)
   w.SetItemDelegateForColumn(1, d)
   w.SetItemDelegateForColumn(2, d)
   
   w.ConnectSelectionChanged(func (add, drop *core.QItemSelection) {
      w.SelectionChangedDefault(add, drop)
      w.RowSelected(len(add.Indexes()) > 0)
   })
   
   w.ConnectDoubleClicked(func (_ *core.QModelIndex) {
      w.ViewSelected()
   })
}

/* SetModel
**    Augments default behaviour to stretch first column (path).
*/

func (w *ReportView) SetModel (m *ResultsModel) {
   if m == nil {
      prev := w.Model()
      if prev != nil && prev.RowCount(core.NewQModelIndex()) == 0 { return }
      m = NewResultsModel(nil)
   }
   
   w.QTableView.SetModel(m)
   w.HorizontalHeader().
      SetSectionResizeMode2(0, widgets.QHeaderView__Stretch)
}

/* ViewSelected
**    Shows a detailed view of the differences (if any) for the currently
** selected row.
*/

func (w *ReportView) ViewSelected () error {
   s := w.SelectedIndexes()
   if len(s) == 0 { return nil }
   path := s[0].Data(int(core.Qt__EditRole)).ToString()
   return ViewFile(path)
}

/*---------------------------------------------------------------------------
   CenteredItemDelegate [type]
---------------------------------------------------------------------------*/

type CenteredItemDelegate struct {
   widgets.QStyledItemDelegate
   
   _ func() `constructor:"init"`
}

/* init
**    Attaches a custom 'paint' function to center text and icon.
*/

func (d *CenteredItemDelegate) init () {
   d.ConnectPaint(d.paint)
}

/* paint
**    Paints as usual but centers output in the table cell.
*/

func (d *CenteredItemDelegate) paint (
   painter *gui.QPainter, option *widgets.QStyleOptionViewItem, index *core.QModelIndex,
) {
   d.InitStyleOption(option, index)
   icon := option.Icon()
   if icon != nil && ! icon.IsNull() {
      rect := option.Rect().Adjusted(5, 5, -5, -5)
      icon.Paint(painter, rect, core.Qt__AlignCenter, gui.QIcon__Normal, gui.QIcon__Off)
   } else {
      option.SetDisplayAlignment(core.Qt__AlignCenter)
      d.PaintDefault(painter, option, index)
   }
}