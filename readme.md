<h3 align=center><b>vi-pipe</b></h3>
<p align=center><i>edit text in the middle of a shell pipeline, and store a diff to replay changes after further invocations</i></p>

---

### installation

```
    go install go.senan.xyz/vi-pipe@latest
```

### usage

```
    export EDITOR=vi
    vi-pipe [-re]
```

### example

```shell
    # there are a lot of possibilities with this thing
    # for example list some files, clean them up or prune them in the editor, then delete them
    $ ls ~/my-files | vi-pipe | xargs rm

    # changes are kept, so long as the current directory doesn't change
    $ echo a | vi-pipe
    ab   # only added the "b" char in editor
    $ echo a | vi-pipe
    abc  # only added the "c" char in editor, previous change pre-applied

    # manipulate some data
    $ cat people.csv | csv-to-json | jq '.[] | .address'
    # oops, no jq output. this csv has no header row, let's add it in the editor
    $ cat people.csv | vi-pipe | csv-to-json | jq '.[] | .address'
    Dublin, Irenand
    Barcelona, Spain
    # nice, but there's a typo. let me re-open in the editor. there'll be no need to add the header again
    $ cat people.csv | vi-pipe -re | csv-to-json | jq '.[] | .address'
    Dublin, Ireland
    Barcelona, Spain
```
