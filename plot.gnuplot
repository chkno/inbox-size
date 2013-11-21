#!/usr/bin/gnuplot

set timefmt "%Y-%m-%d %H:%M:%S"
set xdata time
set grid
set key top left
set ylabel "Emails"
set y2label "Days"
set ytics nomirror
set y2tics
plot "log" using 1:5 with steps lw 2 title "Email count", \
     "< ./time-empty.awk log" using 1:($5 / 86400) with lines title "Time since empty" axes x1y2
pause -1 "Press enter to dismiss"
