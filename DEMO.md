# Nimbus CLI — Feature Demo

A walkthrough of every command from setup to daily use.

---

## Setup

**Install dependencies and build the CLI**

```bash
# Start local services (PostgreSQL + LocalStack S3)
docker compose up -d postgres localstack

# Build the CLI binary
cd client && go build -o nim cli/main.go

# (optional) move it to your PATH so you can run `nim` from anywhere
mv nim /usr/local/bin/nim
```

**Environment**

By default the CLI targets `localhost:8080`. To switch to production:

```bash
export NIM_ENV=prod
export NIM_API_URL=http://your-alb-url.us-east-1.elb.amazonaws.com
```

---

## Authentication

### Register

```bash
# Start the server first
cd server && NIM_ENV=local go run main.go
```

```bash
nim login
```

You will be prompted for your email and password. On success your session (JWT + user info) is cached locally in Redis so you stay logged in across terminal sessions.

### Logout

```bash
nim logout
```

Clears the local session cache. You will need to `nim login` again before running any other command.

---

## Boxes

Boxes are the top-level containers — think of them as drives or buckets. All files and folders live inside a box.

### Create a box

```bash
nim mkbox my-box
```

### List your boxes

```bash
nim bls
```

```
my-box
work-files
photos
```

### Switch to a box (set it as active)

```bash
nim cb my-box
```

All subsequent commands operate inside `my-box` until you switch again.

### Delete a box

```bash
nim rmbox my-box
```

> Deletes the box and **all** files and folders inside it from both S3 and the database.

---

## Navigation

The CLI keeps track of a current working path inside the active box, just like a terminal.

### Print current location

```bash
nim pwd
```

```
my-box/
```

### Change directory

```bash
nim cd documents          # go into a subfolder (relative)
nim cd /documents/work    # jump to an absolute path
nim cd ..                 # go up one level
nim cd                    # go back to the box root
```

```
my-box/documents
```

### List contents

```bash
nim ls                    # list the current directory
nim ls reports            # list a subfolder (relative)
nim ls /documents/work    # list an absolute path
```

```
my-box/documents

  [dir]  work/
  [dir]  personal/
  [file] readme.txt                       1.2 KB
  [file] budget.xlsx                     48.3 KB

  2 folder(s), 2 file(s)
```

You can also use the flag form:

```bash
nim ls --path documents/work
```

---

## Folders

### Create a folder

```bash
nim cdir reports              # create in the current working directory
nim cdir archive reports      # create inside an explicit parent path
```

### Rename a folder

```bash
nim mvdir old-name new-name
```

> Renames the folder in the database and copies all S3 objects to the new key prefix.

### Delete a folder

```bash
nim rmdir reports
```

> Recursively deletes all files and sub-folders from S3 and the database.

---

## Files

### Upload a file

```bash
nim post --file ./invoice.pdf
nim post --file ./notes.txt --destination documents/work
```

The CLI requests a short-lived S3 presigned PUT URL from the server, then streams the file directly to S3 — the file bytes never pass through the API server.

### Download a file

```bash
nim get --file users/nim-user-1/boxes/my-box/invoice.pdf
nim get --file users/nim-user-1/boxes/my-box/invoice.pdf --output ./downloads/invoice.pdf
```

Same pattern in reverse: server issues a presigned GET URL, CLI streams bytes directly from S3.

### Rename a file

```bash
nim rename \
  --key  users/nim-user-1/boxes/my-box/notes.txt \
  --name new_notes.txt
```

### Move a file

```bash
nim mv \
  --key users/nim-user-1/boxes/my-box/notes.txt \
  --to  documents/work
```

Moves the file to `documents/work/` inside the active box. Leave `--to` empty to move it to the box root.

### Delete a file

```bash
nim del --file users/nim-user-1/boxes/my-box/old_report.pdf
```

---

## Typical Workflow

```bash
# 1. Log in
nim login

# 2. Create a box and navigate into it
nim mkbox work-files
nim cb work-files

# 3. Create some folders
nim cdir documents
nim cdir archive
nim cd documents

# 4. Upload files
nim post --file ./q1-report.pdf
nim post --file ./notes.txt

# 5. Check what's there
nim ls

# 6. Reorganise
nim mv --key users/nim-user-1/boxes/work-files/notes.txt --to archive
nim mvdir documents docs

# 7. Download a file
nim get --file users/nim-user-1/boxes/work-files/q1-report.pdf --output ./q1-report.pdf

# 8. Clean up
nim rmdir archive
nim logout
```
