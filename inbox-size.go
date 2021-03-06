package main

import (
	"bufio"
	"container/list"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"math"
	"math/rand"
	"os"
	"strings"
	"time"
)

var flag_server = flag.String("server", "imap.gmail.com:993", "Machine and port to connect to")
var flag_credentialsfile = flag.String("credentialsfile", ".inbox-size-credentials", "File with username and password separated by a space")
var flag_mailbox = flag.String("mailbox", "INBOX", "Mailbox to monitor")
var flag_interval = flag.String("interval", "1m", "Wait this long between polls")
var flag_timeout = flag.String("timeout", "5m", "Reconnect if a poll takes longer than this")
var flag_verbose = flag.Bool("verbose", false, "Show the IMAP chatter")

type Options struct {
	Server, Username, Password, Mailbox string
	Interval, Timeout                   time.Duration
	Verbose                             bool
}

func imap_quote(x string) string {
	x = strings.Replace(x, "\\", "\\\\", -1)
	x = strings.Replace(x, "\"", "\\\"", -1)
	x = strings.Replace(x, "/", "\\/", -1)
	return "\"" + x + "\""
}

func load_options_from_flags() (*Options, error) {
	var opts Options

	opts.Server = *flag_server
	opts.Mailbox = *flag_mailbox
	opts.Verbose = *flag_verbose

	credentialbytes, err := ioutil.ReadFile(*flag_credentialsfile)
	if err != nil {
		return nil, errors.New("Couldn't read credential file " +
			*flag_credentialsfile + ": " + err.Error())
	}
	credentials := strings.SplitN(strings.TrimSpace(string(credentialbytes)), " ", 2)
	opts.Username = credentials[0]
	opts.Password = credentials[1]

	opts.Interval, err = time.ParseDuration(*flag_interval)
	if err != nil {
		return nil, errors.New("Couldn't parse interval value " +
			*flag_interval + ": " + err.Error())
	}

	opts.Timeout, err = time.ParseDuration(*flag_timeout)
	if err != nil {
		return nil, errors.New("Couldn't parse timeout value " +
			*flag_timeout + ": " + err.Error())
	}

	return &opts, nil
}

func send_command(conn *tls.Conn, scanner *bufio.Scanner, opts *Options, tag, command string) error {
	conn.SetDeadline(time.Now().Add(opts.Timeout))
	conn.Write([]byte(tag + " " + command + "\r\n"))
	if opts.Verbose {
		fmt.Fprintln(os.Stderr, ">>> "+tag+" "+command)
	}
	for {
		if !scanner.Scan() {
			if scanner.Err() == nil {
				return errors.New("Unexpected EOF while reading from server")
			}
			return errors.New("Error while reading from server: " + scanner.Err().Error())
		}
		if opts.Verbose {
			fmt.Fprintln(os.Stderr, "<<< ", scanner.Text())
		}
		if strings.HasSuffix(scanner.Text(), "EXISTS") {
			fmt.Println(time.Now(), strings.Split(scanner.Text(), " ")[1])
			os.Stdout.Sync()
		}
		if strings.HasPrefix(scanner.Text(), tag+" ") {
			if strings.HasPrefix(scanner.Text(), tag+" OK ") {
				break
			}
			return errors.New("Server returned error: " + scanner.Text())
		}
	}
	return nil
}

func run_until_error(opts *Options) error {
	conn, err := tls.Dial("tcp", opts.Server, nil)
	if err != nil {
		return err
	}
	scanner := bufio.NewScanner(conn)

	if err = send_command(conn, scanner, opts, "login", "LOGIN "+imap_quote(opts.Username)+" "+imap_quote(opts.Password)); err != nil {
		return err
	}

	if err = send_command(conn, scanner, opts, "examine", "EXAMINE "+imap_quote(opts.Mailbox)); err != nil {
		return err
	}

	seq := 0
	for {
		time.Sleep(opts.Interval)
		seq++
		tag := fmt.Sprintf("a%d", seq)
		if err = send_command(conn, scanner, opts, tag, "NOOP"); err != nil {
			return err
		}
	}
}

func random_exponential_backoff(attempt_times *list.List) {
	const (
		window_size = 1 * time.Hour
		max_delay   = 5 * time.Minute
	)
	now := time.Now()
	attempt_times.PushBack(now)
	discard_point := now.Add(-window_size)
	for attempt_times.Front().Value.(time.Time).Before(discard_point) {
		attempt_times.Remove(attempt_times.Front())
	}
	if attempt_times.Len() > 1 {
		delay := time.Duration(
			float64(200*time.Millisecond) *
				math.Pow(2, float64(attempt_times.Len())) *
				rand.Float64())
		if delay > max_delay {
			delay = max_delay
		}
		fmt.Fprint(os.Stderr, "Waiting ", delay, " before retry...")
		time.Sleep(delay)
		fmt.Fprintln(os.Stderr)
	}
}

func run_continuously(opts *Options) {
	attempt_times := list.New()
	for {
		random_exponential_backoff(attempt_times)
		err := run_until_error(opts)
		fmt.Fprintln(os.Stderr, err)
	}
}

func main() {
	flag.Parse()
	rand.Seed(time.Now().UnixNano())
	opts, err := load_options_from_flags()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	run_continuously(opts)
}
