#!/usr/bin/awk -f

# How long has it been since the inbox has been empty?

function parsetime(d, t) {
  return mktime(gensub("\\..*", "", "", gensub("[-:]", " ", "g", d " " t)))
}

prev == 0 && $5 > 0 {
  start = parsetime($1, $2)
  print $1, $2, $3, $4, 0
}

start && $5 == 0 {
  end = parsetime($1, $2)
  print $1, $2, $3, $4, end - start
  print $1, $2, $3, $4, 0
  start = 0
}

{
  prev = $5
}

END {
  if (start) {
    print strftime("%Y-%m-%d %H:%M:%S.%N %z %Z"), systime() - start
  }
}
