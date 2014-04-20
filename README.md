# Laracast download

This is a small utility to download all lessons from http://laracasts.com
written in [Go](http://golang.org/).

## Running the scraper

``` bash
go run laracasts-dl.go myusername mypassword dest/
```

Or:

``` bash
go build laracasts-dl.go
./laracasts-dl myusername mypassword dest/
```

## Example / screenshots

``` bash
$ laracasts-dl myusername mypassword
2014/04/19 23:47:50 Logging in
2014/04/19 23:47:51 Found 148 lessons
2014/04/19 23:47:51 Downloading lesson 1/148 (A Tour of the Laracasts Source)
50.36 MB / 50.66 MB [===================================] 99.41 % 3.22 MB/s 2014/04/19 23:48:08
Downloading lesson 2/148 (Important Breaking Change in 4.1.26)
12.38 MB / 12.44 MB [===================================] 99.52 % 3.25 MB/s 2014/04/19 23:48:12
Downloading lesson 3/148 (Maybe You Should Use SQLite)
20.53 MB / 20.90 MB [===================================] 98.21 % 3.30 MB/s 2014/04/19 23:48:20
Downloading lesson 4/148 (Enforcement, Entities, and Eloquent)
46.26 MB / 46.74 MB [===================================] 98.98 % 2.72 MB/s 2014/04/19 23:48:38
Downloading lesson 5/148 (PHP 5.6 in 10 Minutes)
15.40 MB / 15.93 MB [===================================] 96.64 % 3.20 MB/s 2014/04/19 23:48:43
Downloading lesson 6/148 (Entities vs. Value Objects)
40.88 MB / 41.39 MB [===================================] 98.76 % 3.24 MB/s 2014/04/19 23:48:57
Downloading lesson 7/148 (Supervise This)
9.98 MB / 10.15 MB [====================================] 98.30 % 3.32 MB/s 2014/04/19 23:49:01
Downloading lesson 8/148 (The Failed Job Interrogation)
28.19 MB / 28.48 MB [===================================] 98.99 % 3.20 MB/s 2014/04/19 23:49:11
Downloading lesson 9/148 (Beanstalkd Queues)
32.30 MB / 32.70 MB [===================================] 98.79 % 3.22 MB/s 2014/04/19 23:49:22
Downloading lesson 10/148 (How to use Eloquent Outside of Laravel)
16.47 MB / 16.80 MB [===================================] 98.06 % 3.16 MB/s 2014/04/19 23:49:28
Downloading lesson 11/148 (Form Macros for the Win)
50.83 MB / 83.88 MB [=================>-----------------] 60.60 % 2.98 MB/s 11s
```

[![screenshot](https://github.com/LeonB/laracasts-dl/raw/master/screenshot.png)](https://github.com/LeonB/laracasts-dl/raw/master/screenshot.png)
