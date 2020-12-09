package app

/*
** This file contains the logic to handle the cache of file "fingerprints".
*/

import (
   "encoding/gob"
   "path/filepath"
   "os"
   "log"
   "time"
   "sort"
)

/*---------------------------------------------------------------------------
   FilePrint [type]
      Stores the details for both local and remote copies of a given file or
   folder.
---------------------------------------------------------------------------*/

type FilePrint struct {
   Local,
   Remote      FileInfo
}

type FileInfo struct {
   IsDir,
   Changed     bool
   ModTime     time.Time
   Size        int64
   Hash        []byte
}

/*---------------------------------------------------------------------------
   Cache [type]
      In-memory copy of file fingerprint cache. Each entry is stored as a
   Go map, in which the key is the relative path of the file from the local
   or remote root.
---------------------------------------------------------------------------*/

type Cache struct {
   // public fields - saved to disk:
   FilePrints  map[string]*FilePrint
   
   // private fields:
   path        string
}

/*---------------------------------------------------------------------------
   NewCache
      Creates and returns an 'empty' cache file for the current site.
---------------------------------------------------------------------------*/

func NewCache () *Cache {
   path := Config.CacheFile
   if path != "" && filepath.Base(path) == path {
      path = filepath.Join(Config.Source, path)
   }
   return &Cache{
      FilePrints: make(map[string]*FilePrint),
      path:       path,
   }
}

/*---------------------------------------------------------------------------
   LoadCache
      Reads and decodes the saved cache file from the last run. If that file
   does not exists then a new, empty cache is created.
---------------------------------------------------------------------------*/

func LoadCache () (*Cache, error) {
   cache := NewCache()
   
   f, err := os.Open(cache.path)
   if err != nil {
      if ! os.IsNotExist(err) { return nil, err }
      if Opt.Verbose { log.Println("Cache file not found; creating new cache") }
      return cache, nil
   }
   defer f.Close()
   
   if Opt.Verbose { log.Println("Loading saved cache") }
   
   dec := gob.NewDecoder(f)
   err = dec.Decode(cache)
   if err != nil { return nil, err }
   
   return cache, nil
}

/*---------------------------------------------------------------------------
   Cache::Write
      Encodes and saves the file fingerprint cache that resulted from this
   run.
---------------------------------------------------------------------------*/

func (cache *Cache) Write () {
   err := Config.Check()
   if err != nil { return }
   
   f, err := os.Create(cache.path)
   if err == nil {
      defer f.Close()

      if Opt.Verbose { log.Println("Writing new cache file") }

      enc := gob.NewEncoder(f)
      err = enc.Encode(cache)
   }
   
   if err != nil {
      qMain.showError("Write cache", err)
   }
}

/*---------------------------------------------------------------------------
   Cache::AddEntry
      Adds or retrieves a fingerprint entry for a file or folder.
---------------------------------------------------------------------------*/

func (cache *Cache) AddEntry (path string) *FilePrint {
   ent, ok := cache.FilePrints[path]
   if ! ok {
      ent = new(FilePrint)
      ent.Local.Changed = true
      ent.Remote.Changed = true
      cache.FilePrints[path] = ent
   }
   return ent
}

/*---------------------------------------------------------------------------
   Cache::Keys
      Returns a sorted list of the file paths for which there are local or
   remote fingerprints.
---------------------------------------------------------------------------*/

func (cache *Cache) Keys () []string {
   keys := make([]string, len(cache.FilePrints))
   n := 0;
   for k := range cache.FilePrints { keys[n] = k; n++ }
   if len(keys) > 1 { sort.Strings(keys) }
   return keys
}

/*---------------------------------------------------------------------------
   Cache::Walk
      Walks through the cache, calling a callback function for each entry.
---------------------------------------------------------------------------*/

func (cache *Cache) Walk (cb func (string, *FilePrint)) {
   keys := cache.Keys()
   for _, k := range keys { cb(k, cache.FilePrints[k]) }
}