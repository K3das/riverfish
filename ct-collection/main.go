package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	ct "github.com/K3das/ct-streamer"
	"github.com/K3das/riverfish/proto/go/pipeline"
	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
)

func initTopic() {
	pandaConn, err := kafka.Dial("tcp", os.Getenv("PANDA"))
	if err != nil {
		log.Fatalln(err)
	}
	defer pandaConn.Close()

	controller, err := pandaConn.Controller()
	if err != nil {
		log.Fatalln(err)
	}
	var controllerConn *kafka.Conn
	controllerConn, err = kafka.Dial("tcp", net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port)))
	if err != nil {
		log.Fatalln(err)
	}
	defer controllerConn.Close()

	err = controllerConn.CreateTopics(kafka.TopicConfig{
		Topic:             os.Getenv("INGEST_TOPIC"),
		NumPartitions:     -1,
		ReplicationFactor: -1,
		ConfigEntries: []kafka.ConfigEntry{
			{ConfigName: "retention.ms", ConfigValue: fmt.Sprint(1000 * 60 * 5)},
		},
	})
	if err != nil {
		log.Fatalln(err)
	}
}

func main() {
	initTopic()

	pandaWriter := &kafka.Writer{
		Addr:                   kafka.TCP(os.Getenv("PANDA")),
		Topic:                  os.Getenv("INGEST_TOPIC"),
		Balancer:               &kafka.LeastBytes{},
		AllowAutoTopicCreation: false,
	}
	defer pandaWriter.Close()

	httpClient := http.Client{
		Timeout: time.Second * 5,
	}

	streamer, err := ct.NewStreamer(&httpClient)
	if err != nil {
		log.Fatal(err)
	}

	if err := streamer.Run(context.Background(), func(batch ct.ResultLogs) error {
		var messages []kafka.Message
		for _, l := range batch.Certificates {
			names := make(map[string]struct{})
			for _, v := range l.DNSNames {
				name := v
				// if len(v) > 2 && name[:2] == "*." {
				// 	name = name[2:]
				// }
				if _, ok := names[name]; ok {
					if _, ok := names["*."+name]; ok {
						continue
					} else {
						log.Printf("duplicate %s", name)
					}
					continue
				}
				names[name] = struct{}{}

				messageValue, err := proto.Marshal(&pipeline.CertificateLog{
					Subject: name,
					Index:   l.Index,
					LogUrl:  batch.LogURL,
				})
				if err != nil {
					log.Printf("error serializing %s (%d): %s", v, l.Index, err.Error())
					continue
				}
				messages = append(messages, kafka.Message{
					Key:   []byte(uuid.New().String()),
					Value: messageValue,
				})
			}
		}
		err = pandaWriter.WriteMessages(context.Background(), messages...)
		if err != nil {
			log.Printf("error writing to panda: %s", err.Error())
		}
		return nil
	}); err != nil {
		log.Fatal(err)
	}
}
