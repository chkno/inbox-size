package main

import (
	"bufio"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
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

// TODO: Quote username, password, and mailbox name
// Currently, things break if they contain special characters

type Options struct {
	Server, Credentials, Mailbox string
	Interval, Timeout            time.Duration
	Verbose                      bool
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
	opts.Credentials = strings.TrimSpace(string(credentialbytes))

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

func run_until_error(opts *Options) error {
	conn, err := tls.Dial("tcp", opts.Server, nil)
	if err != nil {
		panic(err)
	}
	conn.SetDeadline(time.Now().Add(opts.Timeout))
	scanner := bufio.NewScanner(conn)
	conn.Write([]byte("login LOGIN " + opts.Credentials + "\r\n"))
	if opts.Verbose {
		fmt.Fprintln(os.Stderr, ">>> login LOGIN <credentials>")
	}
	for {
		if !scanner.Scan() {
			if scanner.Err() == nil {
				return errors.New("Unexpected EOF while reading from server")
			}
			return errors.New("While reading from server: " + scanner.Err().Error())
		}
		if opts.Verbose {
			fmt.Fprintln(os.Stderr, "<<< ", scanner.Text())
		}
		if strings.HasPrefix(scanner.Text(), "login ") {
			if strings.HasPrefix(scanner.Text(), "login OK ") {
				break
			}
			return errors.New("Login error: " + scanner.Text())
		}
	}
	conn.SetDeadline(time.Now().Add(opts.Timeout))
	conn.Write([]byte("examine EXAMINE " + opts.Mailbox + "\r\n"))
	if opts.Verbose {
		fmt.Fprintln(os.Stderr, ">>> examine EXAMINE "+opts.Mailbox)
	}
	for {
		if !scanner.Scan() {
			if scanner.Err() == nil {
				return errors.New("Unexpected EOF while reading from server")
			}
			return errors.New("While reading from server: " + scanner.Err().Error())
		}
		if opts.Verbose {
			fmt.Fprintln(os.Stderr, "<<< ", scanner.Text())
		}
		if strings.HasSuffix(scanner.Text(), "EXISTS") {
			fmt.Println(time.Now(), strings.Split(scanner.Text(), " ")[1])
		}
		if strings.HasPrefix(scanner.Text(), "examine ") {
			if strings.HasPrefix(scanner.Text(), "examine OK ") {
				break
			}
			return errors.New("Error opening mailbox " + opts.Mailbox + ": " + scanner.Text())
		}
	}
	seq := 0
	for {
		time.Sleep(opts.Interval)
		seq++
		tag := fmt.Sprintf("a%d", seq)
		conn.SetDeadline(time.Now().Add(opts.Timeout))
		conn.Write([]byte(tag + " NOOP\r\n"))
		if opts.Verbose {
			fmt.Fprintln(os.Stderr, ">>> "+tag+" NOOP")
		}
		for {
			if !scanner.Scan() {
				if scanner.Err() == nil {
					return errors.New("Unexpected EOF while reading from server")
				}
				return errors.New("While reading from server: " + scanner.Err().Error())
			}
			if opts.Verbose {
				fmt.Fprintln(os.Stderr, "<<< ", scanner.Text())
			}
			if strings.HasSuffix(scanner.Text(), "EXISTS") {
				fmt.Println(time.Now(), strings.Split(scanner.Text(), " ")[1])
			}
			if strings.HasPrefix(scanner.Text(), tag+" ") {
				if strings.HasPrefix(scanner.Text(), tag+" OK") {
					break
				}
				return errors.New("Error from server: " + scanner.Text())
			}
		}
	}
}

func run_continuously(opts *Options) {
	for {
		// TODO: Randomized exponential backoff
		err := run_until_error(opts)
		fmt.Println(err)
	}
}

func main() {
	flag.Parse()
	opts, err := load_options_from_flags()
	if err != nil {
		fmt.Println(err)
		return
	}
	run_continuously(opts)
}
