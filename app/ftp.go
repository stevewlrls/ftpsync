package app

import (
   "fmt"
   "log"
   "os"
   "io"
   "net"
   "bytes"
   "errors"
   "time"
   "crypto/tls"
   "crypto/x509"
   "github.com/secsy/goftp"
   "github.com/pkg/sftp"
   "golang.org/x/crypto/ssh"
   "github.com/therecipe/qt/widgets"
)

/*---------------------------------------------------------------------------
   FTPConn [interface]
      Common interface for both FTP and SFTP connections.
---------------------------------------------------------------------------*/

type FTPConn interface {
   Close () error
   ReadDir (string) ([]os.FileInfo, error)
   Retrieve (string, io.Writer) error
}

/*---------------------------------------------------------------------------
   DialRemote
      Detects the type of FTP/SFTP connection from the remote URL 'scheme'
   and returns an active connection that supports the abstract interface
   above.
---------------------------------------------------------------------------*/

func DialRemote () (FTPConn, error) {
   switch (Config.RemoteAddr.Scheme) {
      case "ftp", "ftps": {
         return dialFTP()
      }
      case "sftp": {
         return dialSFTP()
      }
      default: {
         return nil, E_BadScheme()
      }
   }
}

/*---------------------------------------------------------------------------
   E_BadScheme
      Returns customised error for unsupported scheme.
---------------------------------------------------------------------------*/

func E_BadScheme () error {
   return fmt.Errorf("Unsupported scheme (%s) for remote address", Config.RemoteAddr.Scheme)
}

/*---------------------------------------------------------------------------
   dialFTP
      Helper function to create and return a normal FTP session (with or
   without TLS).
---------------------------------------------------------------------------*/

func dialFTP () (FTPConn, error) {
   var err error
   if Opt.Verbose { log.Println("Opening FTP session") }
   
   user := Config.RemoteAddr.User.Username()
   pwd, ok := Config.RemoteAddr.User.Password()
   if ! ok {
      pwd = Config.password
      if pwd == "" {
         pwd, err = promptForPassword()
         if err != nil { return nil, err }
         Config.password = pwd
      }
   }
   
   config := goftp.Config{
      User:                user,
      Password:            pwd,
      ConnectionsPerHost:  1,
      Timeout:             time.Second * 20,
   }
   
   if Config.RemoteAddr.Scheme == "ftps" {
      config.TLSConfig = &tls.Config{
         ServerName:             Config.RemoteAddr.Host,
         VerifyPeerCertificate:  vetServerTrust,
         InsecureSkipVerify:     true,
      }
   }
   
   conn, err := goftp.DialConfig(config, Config.RemoteAddr.Host)
   if err != nil { return nil, err }
   
   _, err = conn.ReadDir(Config.RemoteAddr.Path)
   if err != nil {
      conn.Close(); return nil, err
   }
   
   return conn, nil
}

/*---------------------------------------------------------------------------
   vetServerTrust
      Called to vet the FTPS server certificate, in place of normal checking
   of trust chains. Attempts to verify the chain of trust from the server's
   given certificates. If the cert is verified then of course we allow it,
   but if not then we ask the user if they want to trust it. This is normally
   the case if the cert is 'self-signed'.
---------------------------------------------------------------------------*/

func vetServerTrust (raw [][]byte, verified [][]*x509.Certificate) error {
   certs, err := x509.ParseCertificates(raw[0])
   if err != nil { return err }
   
   leaf := certs[0]
   if Config.ServerKey != nil && bytes.Equal(Config.ServerKey, leaf.Signature) {
      return nil // already decided to trust
   }
   
   pool := x509.NewCertPool()
   for _, c := range certs[1:] { pool.AddCert(c) }
   
   _, err = leaf.Verify(x509.VerifyOptions{
      DNSName:       Config.RemoteAddr.Host,
      Roots:         nil, // use system roots
      Intermediates: pool,
   })
   if err == nil { return nil }
   
   box := widgets.NewQMessageBox2(
      widgets.QMessageBox__Warning,
      "TLS Certificate",
      fmt.Sprintf(
         "The security certificate from %s could not be verified.\n%s\n",
         Config.RemoteAddr.Host,
         err.Error(),
      ),
      widgets.QMessageBox__Ok | widgets.QMessageBox__Cancel,
      nil, 0,
   )
   box.SetDetailedText(
      fmt.Sprintf(
         "Certificate details:\n  Subject: %s\n  Issuer:  %s\n",
         leaf.Subject.String(),
         leaf.Issuer.String(),
      ),
   )
   box.SetInformativeText("Do you trust this server?")
   
   answer := box.Exec()
   if answer == int(widgets.QMessageBox__Ok) {
      Config.ServerKey = leaf.Signature
      return nil
   }
   
   return fmt.Errorf("TLS certificate not trusted")
}

/*---------------------------------------------------------------------------
   dialSFTP
      Helper function to create and return an SSH FTP session (SFTP).
---------------------------------------------------------------------------*/

func dialSFTP () (FTPConn, error) {
   var err error
   if Opt.Verbose { log.Println("Opening SFTP session") }
   
   user := Config.RemoteAddr.User.Username()
   pwd, ok := Config.RemoteAddr.User.Password()
   if ! ok {
      pwd, err = promptForPassword()
      if err != nil { return nil, err }
   }
   
   conn, err := ssh.Dial("tcp", Config.RemoteAddr.Host, &ssh.ClientConfig{
      User:             user,
      Auth:             []ssh.AuthMethod{ ssh.Password(pwd) },
      HostKeyCallback:  vetHostKey,
   })
   if err != nil { return nil, err }
   
   client, err := sftp.NewClient(conn)
   if err != nil { return nil, err }
   
   return SFTPConn{client}, nil
}

/*---------------------------------------------------------------------------
   SFTPConn
      Wrapper for SFTP client connection, to implement same interface as
   FTP (above).
---------------------------------------------------------------------------*/

type SFTPConn struct {
   *sftp.Client
}

func (c SFTPConn) Retrieve (path string, dest io.Writer) error {
   f, err := c.Client.Open(path)
   if err != nil { return err }
   defer f.Close()
   
   _, err = io.Copy(dest, f)
   return err
}

/*---------------------------------------------------------------------------
   promptForPassword
      Helper function to prompt the user to enter a password, if this has
   been omitted from the remote URL.
---------------------------------------------------------------------------*/

func promptForPassword () (pwd string, err error) {
   ok := false
   pwd = widgets.QInputDialog_GetText(
      nil,
      "Password",
      "Enter password for " + Config.RemoteAddr.Host,
      widgets.QLineEdit__Password,
      "",
      &ok,
      0, 0,
   )
   if ! ok { err = errors.New("Transfer cancelled") }
   return
}

/*---------------------------------------------------------------------------
   vetHostKey
      Called to vet the SSH server key. If we have connected before, then
   the key should be the same as last time. Otherwise - or if different -
   we need to ask the user if they'll trust this key.
---------------------------------------------------------------------------*/

func vetHostKey (hostname string, remote net.Addr, key ssh.PublicKey) error {
   keyBytes := key.Marshal()
   if Config.ServerKey != nil && bytes.Equal(Config.ServerKey, keyBytes) { return nil }
   
   box := widgets.NewQMessageBox2(
      widgets.QMessageBox__Warning,
      "New SSH Key",
      fmt.Sprintf(
         "The key for %s at %s cannot be verified.\n",
         hostname,
         remote.String(),
      ),
      widgets.QMessageBox__Ok | widgets.QMessageBox__Cancel,
      nil, 0,
   )
   box.SetInformativeText("Do you trust this server?")
   
   answer := box.Exec()
   if answer == int(widgets.QMessageBox__Ok) {
      Config.ServerKey = keyBytes
      return nil
   }
   
   return fmt.Errorf("Server key not trusted (host=%s)", hostname)
}