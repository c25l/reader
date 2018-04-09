I've found that, for the most part, you're better off just using elfeed.

## Bugwilla
...is a small go rss and twitter to org utility with configuration file support.

It handles secrets suboptimally, or, rather, it requests that you do. I find it best if used as a cronjob.

Being written in go it has a lot of interesting trade-offs that make it super undesirable, so, there's that. Emacs would be better for doing the orgmode work, surely, but worse at deployment and the fetching work.
