package connect

import (
	"errors"
	"net"

	"github.com/crunchydata/crunchy-proxy/config"
	"github.com/crunchydata/crunchy-proxy/protocol"
	"github.com/crunchydata/crunchy-proxy/util/log"
)

func Send(connection net.Conn, message []byte) (int, error) {
	return connection.Write(message)
}

func Receive(connection net.Conn) ([]byte, int, error) {
	buffer := make([]byte, 4096)
	length, err := connection.Read(buffer)
	return buffer, length, err
}

func Connect(host string) (net.Conn, error) {
	connection, err := net.Dial("tcp", host)

	if err != nil {
		return nil, err
	}

	if config.GetBool("credentials.ssl.enable") {
		log.Info("SSL is enabled. Determine if connection upgrade is required.")

		/*
		 * First determine if SSL is allowed by the backend. To do this, send an
		 * SSL request. The response from the backend will be a single byte
		 * message. If the value is 'S', then SSL connections are allowed and an
		 * upgrade to the connection should be attempted. If the value is 'N',
		 * then the backend does not support SSL connections.
		 */

		/* Create the SSL request message. */
		message := protocol.NewMessageBuffer([]byte{})
		message.WriteInt32(8)
		message.WriteInt32(protocol.SSLRequestCode)

		/* Send the SSL request message. */
		_, err := connection.Write(message.Bytes())

		if err != nil {
			log.Error("Error sending SSL request to backend.")
			log.Errorf("Error: %s", err.Error())
			return nil, err
		}

		/* Received SSL response message. */
		response := []byte{}
		_, err = connection.Read(response)

		if err != nil {
			log.Error("Error receiving SSL response from backend.")
			log.Errorf("Error: %s", err.Error())
			return nil, err
		}

		/*
		 * If SSL is not allowed by the backend then close the connection and
		 * throw an error.
		 */
		if len(response) > 0 && response[0] != 'S' {
			log.Error("The backend does not allow SSL connections.")
		} else {
			log.Info("SSL connections are allowed.")
			log.Info("Attempting to upgrade connection.")
			//  connection = upgradeClientConnection(node, connection)
			log.Info("Connection successfully upgraded.")
		}
	}

	username := config.GetString("credentials.username")
	database := config.GetString("credentials.database")
	options := config.GetStringMapString("credentials.options")

	startupMessage := protocol.CreateStartupMessage(username, database, options)

	connection.Write(startupMessage)

	response := make([]byte, 2048)
	connection.Read(response)

	authenticated := handleAuthenticationRequest(connection, response)

	if !authenticated {
		return nil, errors.New("Authentication failed")
	}

	return connection, nil
}