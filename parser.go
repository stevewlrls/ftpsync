package main

/*
** This file contains the routines to parse the command line, the format of
** which is as follows:
**
**    ftpsync [flags] [site]
**
** [folder] is the name of the site to be scanned. It defaults to the last
** site selected in the 'settings' dialog.
**
** [flags] is any combination of the following:
**
**    -verbose             Logs activity to standard "error" stream. Mainly
**                         useful for debugging.
*/

import (
   "log"
   "github.com/therecipe/qt/core"
)

var Opt struct {
   Verbose     bool
}

/*---------------------------------------------------------------------------
   ParseOptions
      Called to parse the command line and/or set up defaults for the various
   program options (see above).
---------------------------------------------------------------------------*/

func ParseOptions () {
   parser := core.NewQCommandLineParser()
   verbose := core.NewQCommandLineOption3(
      "verbose", "Logs activity to standard error stream.", "", "",
   )
   
   parser.SetApplicationDescription(
      "Compares a local folder and contents with a remote copy, accessed via FTP.")
   parser.AddHelpOption()
   parser.AddOption(verbose)
   parser.AddPositionalArgument("site", "Site to load as initial default", "name")
   parser.Process(core.QCoreApplication_Arguments())
   
   Opt.Verbose = parser.IsSet2(verbose)
   
   args := parser.PositionalArguments()
   if len(args) > 0 {
      err := Config.Select(args[0])
      if err != nil { log.Fatal(err) }
   }
}