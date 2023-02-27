# Simple load generator for stress-testing EDR software

The purpose of this tool is to measure CPU overhead incurred by having active or passive security monitoring technologies active on a Linux system. Examples are _auditd_, _auditbeat_, _auditd_+[_Laurel_](https://github.com/threathunters-io/laurel) Sysmon for Linux, or any EDR.

The tool spawns trivial processes (`/bin/true`) at a set frequency for a set time and measures user + system CPU usage for a set of given processes.

Example:
```
$ ./edr-loadgen -command /bin/true -delay .005 -duration 30 $(pidof auditd; pidof laurel)
2021/10/20 16:42:34 ./edr-loadgen: exec '/bin/true', every 0.0050 seconds, duration: 30.0000 seconds
2021/10/20 16:42:34 CLK_TCK = 100
2021/10/20 16:43:04 5977 events generated.
2021/10/20 16:43:04 PID 8062: user+sys: 0.43+0.70 = 1.13 seconds / 1.433+2.333 = 3.767 percent
2021/10/20 16:43:04 PID 18249: user+sys: 0.78+0.08 = 0.86 seconds / 2.600+0.267 = 2.867 percent
2021/10/20 16:43:04 SUM: 1.99 seconds / 6.633 percent
```

A CSV report can be generated (`-report FILENAME`). It contains the following fields:
- UNIX timestamp
- number of events (per `-delay`, `-duration` parameters)
- number of events actually generated
- PID
- process command line
- utime, stime, sum (seconds)
- utime, stime, sum (%CPU)

## Author

Hilko Bengen <<bengen@hilluzination.de>>

## License

GPL-3.0, see [LICENSE](LICENSE)
