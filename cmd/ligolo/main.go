package main

import (
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"flag"
	"fmt"
	"github.com/armon/go-socks5"
	"github.com/hashicorp/yamux"
	"github.com/sirupsen/logrus"
	"io"
	"net"
	"strings"
	"time"
)

var tlsFingerprint string

var (
	ErrInvalidServerCert = fmt.Errorf("invalid TLS server certificate")
	ErrInvalidPinnedCert = fmt.Errorf("invalid TLS pinned certificate")
)

func main() {
	fmt.Print(`
██╗     ██╗ ██████╗  ██████╗ ██╗      ██████╗ 
██║     ██║██╔════╝ ██╔═══██╗██║     ██╔═══██╗
██║     ██║██║  ███╗██║   ██║██║     ██║   ██║
██║     ██║██║   ██║██║   ██║██║     ██║   ██║
███████╗██║╚██████╔╝╚██████╔╝███████╗╚██████╔╝
╚══════╝╚═╝ ╚═════╝  ╚═════╝ ╚══════╝ ╚═════╝
              Local Input - Go - Local Output

`)

	bypassVerify := flag.Bool("skipverify", false, "Skip TLS certificate pinning verification")

	targetServer := flag.String("targetserver", "", "The destination server (a RDP client, SSH server, etc.) - when not specified, Ligolo starts a socks5 proxy server")
	relayServer := flag.String("relayserver", "127.0.0.1:5555", "The relay server (the connect-back address)")
	autoRestart := flag.Bool("autorestart", false, "Attempt to reconnect in case of an exception")

	flag.Parse()

	if tlsFingerprint == "" && *bypassVerify == false {
		logrus.Fatal("TLS Fingerprint is missing ! Use -skipverify option to bypass TLS verification")
	}
	for {
		err := StartLigolo(*relayServer, *targetServer, *bypassVerify)
		if err != nil {
			if *autoRestart {
				logrus.Error(err)
			} else {
				logrus.Fatal(err)
			}
		}
		logrus.Warning("Restarting Ligolo...")
		time.Sleep(10 * time.Second)
	}
}

func StartLigolo(relayServer string, targetServer string, skipVerify bool) error {
	var socks *socks5.Server
	logrus.Infoln("Connecting to relay server...")
	config := &tls.Config{InsecureSkipVerify: true}
	conn, err := tls.Dial("tcp", relayServer, config)
	if err != nil {
		return err
	}

	if !skipVerify {
		err := verifyTlsCertificate(conn.ConnectionState())
		if err != nil {
			logrus.WithFields(logrus.Fields{"remoteaddr": conn.RemoteAddr().String()}).Error(err)
			return err
		}
	}

	if targetServer == "" {
		socks, err = startSocksProxy()
		if err != nil {
			logrus.Error("Could not start SOCKS5 proxy !")
			return err
		}
	}

	session, err := yamux.Client(conn, nil)
	if err != nil {
		return err
	}

	logrus.Infoln("Waiting for connections....")

	for {
		stream, err := session.Accept()
		if err != nil {
			return err
		}
		logrus.WithFields(logrus.Fields{"active_sessions": session.NumStreams()}).Println("Accepted new connection !")
		// When no targetServer are specified, starts a socks5 proxy
		if targetServer == "" {
			go socks.ServeConn(stream)
		} else {
			proxyConn, err := net.Dial("tcp", targetServer)
			if err != nil {
				logrus.Errorf("Error creating Proxy TCP connection ! Error : %s\n", err)
				return err
			}
			go handleRelay(stream, proxyConn)
		}

	}
}

func startSocksProxy() (*socks5.Server, error) {
	conf := &socks5.Config{}
	socks, err := socks5.New(conf)
	if err != nil {
		logrus.Error("Could not start SOCKS5 proxy !")
		return nil, err
	}
	return socks, nil
}

func verifyTlsCertificate(connState tls.ConnectionState) error {
	valid := false
	pinnedCert := strings.Replace(tlsFingerprint, ":", "", -1)
	pinnedCertBytes, err := hex.DecodeString(pinnedCert)
	if err != nil {
		return ErrInvalidPinnedCert
	}
	for _, peerCert := range connState.PeerCertificates {
		hash := sha256.Sum256(peerCert.Raw)
		if bytes.Compare(hash[:], pinnedCertBytes) == 0 {
			valid = true
		}
	}
	if !valid {
		return ErrInvalidServerCert
	}
	return nil
}

func handleRelay(src net.Conn, dst net.Conn) {
	stop := make(chan bool, 2)

	go relay(src, dst, stop)
	go relay(dst, src, stop)

	select {
	case <-stop:
		return
	}
}

func relay(src net.Conn, dst net.Conn, stop chan bool) {
	io.Copy(dst, src)
	dst.Close()
	src.Close()
	stop <- true
	return
}
