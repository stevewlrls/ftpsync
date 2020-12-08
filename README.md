# ftpsync

The purpose of **ftpsync** is to compare a local copy of a folder and contents (such as a web site) with a remote copy that is reachable via FTP, FTPS or SFTP. Whilst it uses the file modification times and size in bytes to detect changes, it then computes a "hash" of the actual file contents to detect whether the file has really changed.

The program was created because it was found that some "web hosting" services perform actions that change the last modification time for files, without actually changing their content. This makes it quite hard for an administrator to see whether a remote file has been tampered with. If **ftpsync** says the file has changed then it very probably has.

## Usage

The **ftpsync** program runs as a graphical application and is normally launched from a start menu shortcut (Windows), desktop shortcut (Linux) or applications folder package (MacOS). However, it can also be launched from the command line.

The general format of the command line is as follows:

        ftpsync [flags]

## Flags

`-site=[sitename]`

> This flag pre-selects a named "site" (see below). In the current version of the program, this is not a particularly useful action, though it does pre-load the results from the last scan. If absent, the **ftpsync** program will default to the last site used.

`-verbose`

> Outputs information about the progress of the operation, to the 'standard error' stream, to assist in diagnosing the cause of errors.

## Sites

A "site" links a local folder (and its contents) with a corresponding remote folder. The latter is assumed to be hosted on a server "in the cloud", with access via one of the following methods:

* Plain FTP
* Encrypted FTP - aka FTPS
* Secure shell FTP - aka SFTP

Note that plain FTP should be avoided, as the username and login password (if any) will be sent as clear text, as will the content of all files that are fetched from the server in order to compute a 'fingerprint' or to show differences. However, some 'web hosting' services do not support either of the other two options. Please do try SFTP if FTPS is rejected, though, as many services do support 'SSH' access, upon which SFTP is based.

Encrypted FTP (aka FTPS) is implemented as FTP over TLS with explicit negotiation of the TLS encryption after the basic FTP session has been opened. This is normally a fairly standard process but you might be asked to approve or 'trust' the server certificate the first time you connect, if the chain of trust cannot be traced to a trusted root.

Secure shell FTP (aka SFTP) will also ask for verification that you trust the server, the first time you connect - even if you have used SSH before. Also, the **ftpsync** program does not yet support certificate based login, so you will need to provide a username and a password.

Note that it is recommended to leave the password field blank when defining a site. The **ftpsync** program will then prompt for entry, each time it is run (but only once per run). This avoids saving the password on disk. However, the format of the saved site list is highly "opaque" and if you have chosen a good, strong password, it will not be immediately apparent amongst the other data.

## File Types

By default, the **ftpsync** program treats any unknown file type as 'plain text'. When computing a checksum or 'fingerprint', it will convert the different platform standards for a 'line ending' to a common value. The following file types are treated as binary files and are not translated as part of
the checksum process.

* Common image files (`.jpg` `.jpeg` `.png` `.gif`)
* Common media files (`.mp4` `.mp4` `.ogg` `.mkv`)
* Common packed archive files (`.zip` `.jar` `.phar`)
* Any file with no file type extension

Using the 'advanced' options for a site, the above list can be edited to add additional binary file types. However, the only reason for doing so is to reduce the time spent computing the checksum.

## Exclusion

By default, the **ftpsync** program will compute a fingerprint for every file that it finds, with the exclusion of the following:

* Any pattern listed in a '.gitignore' file
* Any folder called '.git', '_vti_cnf' or '_vti_pvt'
* Any file called '.DS_Store' or 'thumbs.db'

The '_vti_*' folders are produced by older Microsoft tools for creating web sites and since they are marked as 'hidden' and are not published to a remote server by those tools, the user is not likely to be aware of their presence.

Both 'thumbs.db' and '.DS_Store' hold thumbnail preview images on Windows and MacOS, respectively. Again, they are hidden and are unlikely to be consistent between local and remote copies.

The '.git' folder is used by the Git version control system and would not normally be published to a remote server.

The '.gitignore' file (if present) identifies files and folders that should not be considered part of the "product" when the site is managed with the Git version control system.

Using the 'advanced' options for a site, the list of exclusions can be edited to remove any of the above defaults or to add new patterns. Any pattern starting with the character '@' is taken as the name of a file, each line of which defines an additional pattern, whose matching files and folders will be excluded. Each pattern may contain a single '*' wildcard that matches any sequence of non-separator characters.

## Acknowledgements

The **ftpsync** program relies on the standard Go libraries, as well as the following:

* `github.com/secsy/goftp`
* `github.com/pkg/sftp`
* `github.com/therecipe/qt`

For the Windows platform, the WiX Toolset is used to create the 'msi' installer and the build chain relies on the MinGW (Minimal GNU for Windows) tools. Please note that whilst the Qt binding for Go will install parts of the MinGW toolset, it does not install the GNU 'make' tool.