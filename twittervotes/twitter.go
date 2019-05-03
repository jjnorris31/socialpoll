package main

import (
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/garyburd/go-oauth/oauth"
	"github.com/joeshaw/envdecode"
)

var conn net.Conn

/**
 *
 */
func dial(netw, addr string) (net.Conn, error) {
	// conexión fallida, cerrar
	if conn != nil {
		conn.Close()
		conn = nil
	}
	// conectar cada 5 minutos
	netc, err := net.DialTimeout(netw, addr, 5*time.Second)

	if err != nil {
		return nil, err
	}
	// con se mantiene actualizado
	conn = netc

	return netc, nil
}

var reader io.ReadCloser

/**
 * Cierra la conexión, dah
 */
func closeConnection() {

	if conn != nil && reader != nil {
		_ = conn.Close()
		_ = reader.Close()
	}
}

var (
	authClient *oauth.Client
	creds      *oauth.Credentials
)

func setupTwitterAuth() {

	// envdecode -> llena las structs con variables de entorno, evita que estén expuestas y llenarlas una y otra vez
	var credentials struct {
		// credenciales de desarrollador en ~/.bashrc
		ConsumerKey    string `env:"SP_TWITTER_KEY,required"`
		ConsumerSecret string `env:"SP_TWITTER_SECRET,required"`
		AccessToken    string `env:"SP_TWITTER_ACCESSTOKEN,required"`
		AccessSecret   string `env:"SP_TWITTER_ACCESSSECRET,required"`
	}


	if err := envdecode.Decode(&credentials); err != nil {
		log.Fatalln(err)
	}

	// go-auth -> autoriza peticiones con OAuth 1.0
	creds = &oauth.Credentials{
		Token:  credentials.AccessToken,
		Secret: credentials.AccessSecret,
	}

	authClient = &oauth.Client{
		Credentials: oauth.Credentials{
			Token:  credentials.ConsumerKey,
			Secret: credentials.ConsumerSecret,
		},
	}
}

var (
	authSetupOnce sync.Once
	httpClient    *http.Client
)

func makeRequest(req *http.Request, params url.Values) (*http.Response, error) {
	authSetupOnce.Do(func() {
		setupTwitterAuth()
		httpClient = &http.Client{
			Transport: &http.Transport{
				Dial: dial,
			},
		}
	})

	formEnc := params.Encode()
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Content-Length", strconv.Itoa(len(formEnc)))
	req.Header.Set("Authorization",
		authClient.AuthorizationHeader(creds, "POST", req.URL, params))
	return httpClient.Do(req)
}

/**
 * Aquí se guardará únicamente el texto del tuit, no nos importa lo demás
 */
type tweet struct {
	Text string
}

func readFromTwitter(votes chan <- string) {
	// carga las keywords desde MongoDB
	options, err := loadOptions()

	if err != nil {
		log.Println("Mmh, parece que no hemos logrado cargar tus opciones desde MongoDB:", err)
		return
	}
	// url.URL con el stream de twitter
	u, err := url.Parse("https://stream.twitter.com/1.1/statuses/filter.json")

	if err != nil {
		log.Println("algo fue mal con el filtro de Twitter", err)
		return
	}

	query := make(url.Values)
	query.Set("track", strings.Join(options, ","))
	req, err := http.NewRequest("POST", u.String(), strings.NewReader(query.Encode()))

	if err != nil {
		log.Println("algo fue mal con el filtro de Twitter", err)
		return
	}

	// haciendo la petición con la query de opciones separada por comas
	resp, err := makeRequest(req, query)
	if err != nil {
		log.Println("al hacer la petición algo falló:", err)
		return
	}

	reader := resp.Body
	decoder := json.NewDecoder(reader)

	for {
		var tweet tweet
		if err := decoder.Decode(&tweet); err != nil {
			break
		}
		for _, option := range options {
			if strings.Contains(
				strings.ToLower(tweet.Text),
				strings.ToLower(option),
			) {
				log.Println("voto:", option)
				// si el tuit contiene al menos una opción, entonces envíalo al channel votes
				votes <- option
			}
		}
	}
}

func startTwitterStream(stopchannel <- chan struct{}, votes chan <- string) <- chan struct{} {

	stoppedchannel := make(chan struct{}, 1)

	go func() {
		defer func() {
			stoppedchannel <- struct{}{}
		}()
		// loop infinito, sólo se detiene si la orden de detener el channel se dió
		for {
			select {
			case <- stopchannel:
				log.Println("deteniendo el flujo de Twitter...")
				return
			default:
				// si la conexión se pierde, en 10 segundos intenta recuperarla
				log.Println("Recuperando algo cool de Twitter...")
				readFromTwitter(votes)
				log.Println(" (esperando)")
				time.Sleep(10 * time.Second)
			}
		}
	}()
	return stoppedchannel
}