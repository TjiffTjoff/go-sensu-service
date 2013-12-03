package main

import (
	"log"
	"os"
	"path/filepath"
)

func sensu() {
	// Setting up working directory to executable's directory
	// Configuration files should be located under the same directory
	workingDir := filepath.Dir(os.Args[0])

  // Creating or appending to logfile under working directory
  logFile, err := os.OpenFile(workingDir+"/go-sensu.log", os.O_APPEND|os.O_CREATE, 0755)
  if err != nil {
    srvLog.Info("Could not open logfile, exiting")
    stopWork()
  }
  log.SetOutput(logFile)

	clientFile := workingDir + "/client.json"
	rabbitmqFile := workingDir + "/rabbitmq.json"
	checksFile := workingDir + "/checks.json"

	// Parsing configuration files and returns structs
	clientConf, err := ParseClientConfig(clientFile)
	if err != nil {
		log.Printf("Client Configuration: %s\n", err)
		os.Exit(1)
	}
	rabbitmqConf, err := ParseRabbitmqConfig(rabbitmqFile)
	if err != nil {
		log.Printf("Rabbitmq Configuration: %s\n", err)
		os.Exit(1)
	}
	checksConf, err := ParseChecksConfig(checksFile)
	if err != nil {
		log.Printf("Checks Configuration: %s\n", err)
		os.Exit(1)
	}

	// Open an amqp connection to rabbitMQ
	conn, err := OpenConnection(rabbitmqConf)
	if err != nil {
		log.Printf("Connection: %s\n", err)
		os.Exit(1)
	}

	// Open an amqp channel to rabbitMQ for keepalive messages
	keepAlivesAmqpChannel, err := OpenChannel(conn, "keepalives")
	if err != nil {
		log.Printf("Keepalives channel: %s\n", err)
		os.Exit(1)
	}

	// Open an amqp channel to rabbitMQ for check results
	resultsAmqpChannel, err := OpenChannel(conn, "results")
	if err != nil {
		log.Printf("Results channel: %s\n", err)
		os.Exit(1)
	}

	// Channels for communicating keepalive and result messages bodies to sending function
	keepAlivesGoChannel := make(chan []byte)
	resultsGoChannel := make(chan []byte)

	go ListenAndSend("keepalives", keepAlivesGoChannel, keepAlivesAmqpChannel)
	go ListenAndSend("results", resultsGoChannel, resultsAmqpChannel)

	// Start go routine for creating keepalives
  log.Println("Starting to send keepalives")
	go KeepAlive(clientConf, keepAlivesGoChannel)

	// Start go routine for every check in checks configuration
	for checkName, checkConfig := range checksConf {
    log.Printf("Initiating %s\n", checkName)
		go runCheck(clientConf.Name, checkName, checkConfig, resultsGoChannel)
	}

	select {}

}
