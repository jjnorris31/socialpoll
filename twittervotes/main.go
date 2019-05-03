package main

import (
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/bitly/go-nsq"
	"gopkg.in/mgo.v2"
)

var db *mgo.Session

/**
 * Conectando con MongoDB
 */
func dialdb() error {
	var err error
	log.Println("Conectando con MongoDB: localhost")
	db, err = mgo.Dial("localhost")
	return err
}

/**
 * Cerrando la conexión con MongoDB
 */
func closedb() {
	db.Close()
	log.Println("conexión con la base de datos finalizada")
}

type poll struct {
	Options []string
}

/**
 * Método que carga todas las opciones en una poll existente en alguna colección de MongoDB
 */
func loadOptions() ([]string, error) {
	var options []string
	// MongoDrive permite consultas de esta manera
	iter := db.DB("ballots").C("polls").Find(nil).Iter()

	var p poll

	for iter.Next(&p) {
		options = append(options, p.Options...)
	}

	_ = iter.Close()
	return options, iter.Err()
}

/**
 * Publicando en NSQ utilizando el channel votes
 */
func publishVotes(votes <-chan string) <-chan struct{} {

	stopchannel := make(chan struct{}, 1)
	// creando en el puerto asignado NSQ
	pub, _ := nsq.NewProducer("localhost:4150", nsq.NewConfig())

	go func() {
		// al finalizar los votos del channel, detener
		for vote := range votes {
			_ = pub.Publish("votes", []byte(vote))
		}
		log.Println("NSQ publisher: Deteniendo")
		pub.Stop()
		log.Println("NSQ publisher: Detenido")
		stopchannel <- struct{}{}
	}()
	return stopchannel
}

func main() {
	// varias gorutinas pueden acceder a stoplock al mismo tiempo
	var stoplock sync.Mutex
	stop := false
	stopChan := make(chan struct{}, 1)
	signalChan := make(chan os.Signal, 1)

	go func() {

		<-signalChan
		stoplock.Lock()
		stop = true
		stoplock.Unlock()
		log.Println("deteniendo...")
		stopChan <- struct{}{}
		closeConnection()
	}()

	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	if err := dialdb(); err != nil {
		log.Fatalln("vaya, no hemos podido conectar con MongoDB:", err)
	}
	defer closedb()

	votes := make(chan string)
	publisherStoppedChan := publishVotes(votes)
	twitterStoppedChan := startTwitterStream(stopChan, votes)

	go func() {
		for {
			time.Sleep(1 * time.Minute)
			closeConnection()
			stoplock.Lock()
			if stop {
				stoplock.Unlock()
				break
			}
			stoplock.Unlock()
		}
	}()
	<-twitterStoppedChan
	close(votes)
	<-publisherStoppedChan

}