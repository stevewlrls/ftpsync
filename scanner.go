package main

/*
** This file contains the logic to scan the local files and folders and match
** them to remote copies.
*/

import (
   "os"
   "path/filepath"
   "strings"
	"errors"
   "bytes"
   "log"
   "io"
   "bufio"
   "hash"
   "crypto/md5"
)

/*---------------------------------------------------------------------------
   ScanFolders
      Initiates a local and remote scan for the identified source folder and
   corresponding remote path.
---------------------------------------------------------------------------*/

func ScanFolders (cache *Cache, errors chan<- error, stop <-chan bool) {
   qMain.ShowStatus("Opening connection ...")
   
   conn, err := DialRemote()
	if err == nil {
		defer conn.Close()

		s := Scanner{
			Cache:         cache,
			Conn:          conn,
			Local:         Config.Source,
			Remote:        Config.RemoteAddr.Path,
			Exclude:       make([]string, 0, len(Config.Exclude)),
			BinaryFiles:   make(map[string]bool),
		}

		// Expand any 'exclude' patterns that start with '@': each of these
		// refers to a file to read in. Each line of that file (which itself
		// is also ignored) is used as an additional pattern.
		for _, x := range strings.Split(Config.Exclude, "|") {
			if strings.HasPrefix(x, "@") {
				path := x[1:]
				s.Exclude = append(s.Exclude, path)
				if ! filepath.IsAbs(path) { path = filepath.Join(Config.Source, path) }
				f, err := os.Open(path)
				if err != nil { continue }
				scanner := bufio.NewScanner(f)
				for scanner.Scan() { s.Exclude = append(s.Exclude, scanner.Text()) }
				f.Close()
			} else {
				s.Exclude = append(s.Exclude, x)
			}
		}

		if Opt.Verbose { log.Printf("Excluding:    %s\n", s.Exclude) }

		// Make boolean 'map' of binary file extensions
		for _, b := range strings.Split(Config.BinaryFiles, "|") {
			s.BinaryFiles[b] = true
		}

		err = s.Walk(stop)
	}

	errors <- err
	qMain.ScanComplete()
}

/*---------------------------------------------------------------------------
   Scanner [type]
      An object of this type is created to pass the 'constant' data for a
   new scan.
---------------------------------------------------------------------------*/

type Scanner struct {
   Cache       *Cache
   Conn        FTPConn
   Local,
   Remote      string
   Exclude     []string
   BinaryFiles map[string]bool
}

/*---------------------------------------------------------------------------
   Scanner::Walk
      The 'Walk' method traverses the local file tree and compares the files
   and folders with the remote copy.
---------------------------------------------------------------------------*/

func (s *Scanner) Walk (stop <-chan bool) error {
   return filepath.Walk(
      Config.Source,
      func (path string, info os.FileInfo, err error) error {
			select {
				case _ = <-stop:
					return errors.New("Scan aborted")
				default:
					// continue
			}
			
         rel, _ := filepath.Rel(s.Local, path)
   
         switch {
				case err != nil: {
               if os.IsNotExist(err) { return nil }
               return err
            }
            case info.IsDir(): {
               return s.EnterFolder(rel, info)
            }
            default: {
               return s.CheckLocal(rel, info)
            }
         }
      },
   )
}

/*---------------------------------------------------------------------------
   Scanner::EnterFolder
      This method is called when a new folder is entered. It fetches the
   folder contents from the remote copy and creates or updates fingerprints
   for each entry. It also updates "folder" entries created by the above,
   when we know the local copy has the same folder.
---------------------------------------------------------------------------*/

func (s *Scanner) EnterFolder (path string, info os.FileInfo) error {
   if s.excluded(path) { return filepath.SkipDir }
   rel := path; if rel == "." { rel = s.Local }
   if Opt.Verbose { log.Printf("Entering %s\n", rel) }
   qMain.ShowStatus(rel)
   
   // Add this folder to the cache (if not already present).
   ent := s.Cache.AddEntry(path)
   ent.Local = FileInfo{ IsDir: true, ModTime: info.ModTime(), Size: 0 }
   
   // Read remote copy of this folder
   dir, err := s.Conn.ReadDir(filepath.Join(s.Remote, path))
   if err == nil {
      // Check each remote file and update the fingerprint, if necessary.
      for _, inf := range dir {
         rel := filepath.Join(path, inf.Name())
			if s.excluded(rel) { continue }
         if inf.IsDir() {
            ent := s.Cache.AddEntry(rel)
            ent.Remote = FileInfo{
               IsDir: true, ModTime: inf.ModTime(), Size: 0,
            }
         } else {
            s.CheckRemote(rel, inf)
         }
      }
   } else if ent.Remote.IsDir {
      return err // Should be able to read it!
   }
   
   return nil;
}

/*---------------------------------------------------------------------------
   Scanner::CheckLocal
      This method is called for each local file. It checks whether the file
   fingerprint needs to be updated (based on size and last modification time)
   and - if so - recomputes the hash value.
      If there is a corresponding remote file then we will have visited it
   before coming to the local copy and if the remote version has changed
   then the hash for the remote copy will have been computed.
---------------------------------------------------------------------------*/

func (s *Scanner) CheckLocal (path string, info os.FileInfo) error {
   if s.excluded(path) { return nil }
   
	ent := s.Cache.AddEntry(path)
   if ! ent.Local.ModTime.Equal(info.ModTime()) || ent.Local.Size != info.Size() {
      ent.Local.Changed = true
      ent.Local.ModTime = info.ModTime()
      ent.Local.Size = info.Size()
   
      f, err := os.Open(filepath.Join(s.Local, path))
      if err != nil { return err } // Abort scan
      defer f.Close()
      
      hash := md5.New()
      if ! s.isBinary(path) { hash = NewTextHash(hash) }
      
      _, err = io.Copy(hash, f)
      if err != nil {
         if Opt.Verbose { log.Printf("Hash (%s): %v\n", path, err) }
         ent.Local.Hash = nil
         return err
      }
      
      ent.Local.Hash = hash.Sum(nil)
      
      if bytes.Equal(ent.Local.Hash, ent.Remote.Hash) {
         ent.Local.Changed = false
         ent.Remote.Changed = false
      }
   }
   return nil
}

/*---------------------------------------------------------------------------
   Scanner::CheckRemote
      This method is called for each remote file. It checks whether the file
   fingerprint needs to be updated (based on size and last modification time)
   and - if so - recomputes the hash value.
---------------------------------------------------------------------------*/

func (s *Scanner) CheckRemote (path string, info os.FileInfo) error {
   if s.excluded(path) { return nil }
	qMain.ShowStatus(path)
	
   ent := s.Cache.AddEntry(path)
   if ! ent.Remote.ModTime.Equal(info.ModTime()) || ent.Remote.Size != info.Size() {
      ent.Remote.Changed = true
      ent.Remote.ModTime = info.ModTime()
      ent.Remote.Size = info.Size()
      
      // Fetch the file and compute an MD5 hash
      hash := md5.New()
      if ! s.isBinary(path) { hash = NewTextHash(hash) }
      
      err := s.Conn.Retrieve(filepath.Join(s.Remote, path), hash)
      if err != nil {
         if Opt.Verbose { log.Printf("Retrieve (%s): %v\n", path, err) }
         ent.Remote.Hash = nil
         return err
      }
      ent.Remote.Hash = hash.Sum(nil)
   }
   return nil
}

/*---------------------------------------------------------------------------
   excluded
      Helper function that compares a given file path with the patterns in
   the defined 'exclusion' list. Returns 'true' if the file/folder should be
   skipped.
---------------------------------------------------------------------------*/

func (s *Scanner) excluded (path string) bool {
   if path == Config.CacheFile { return true }
   base := filepath.Base(path)
   for _, pattern := range s.Exclude {
      ps := path
      if strings.IndexRune(pattern, filepath.Separator) < 0 { ps = base }
      if matched, _ := filepath.Match(pattern, ps); matched { return true }
   }
   return false
}

/*---------------------------------------------------------------------------
   isBinary
      Helper function that compares a given file type (extension) with the
   built-in and additional user-defined "binary" file types.
---------------------------------------------------------------------------*/

func (s *Scanner) isBinary (path string) bool {
   ext := strings.ToLower(filepath.Ext(path))
   if _, ok := s.BinaryFiles[ext]; ok || ext == "" { return true }
   return false
}

/*---------------------------------------------------------------------------
   NewTextHash
      Returns an object that wraps a crypto hash provider within a function
   that "folds" text line endings to a platform-independent form. This
   produces the same hash value even if an 'ASCII mode' file transfer has
   altered the line ending.
---------------------------------------------------------------------------*/

func NewTextHash (h hash.Hash) hash.Hash {
   return TextHash{h}
}

type TextHash struct {
   hash.Hash      // export underlying has provider
}

func (h TextHash) Write (p []byte) (int, error) {
   b := bytes.ReplaceAll(p, []byte("\r\n"), []byte("\n"))
   _, err := h.Hash.Write(b)
   return len(p), err
}