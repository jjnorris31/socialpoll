# socialpoll

Repositorio que contiene los archivos correspondientes al proyecto final de Bases de Datos Avanzadas.


## Instrucciones

Setear tus credenciales de Twitter Developers en el ~/bin/bash

```
export SP_TWITTER_KEY=...
export SP_TWITTER_SECRET=...
export SP_TWITTER_ACCESSTOKEN=...
export SP_TWITTER_ACCESSSECRET=...
```

Corre NSQ en una terminal

```
nsqlookupd
```

Corre una conexión TCP para la query de NSQ

```
nsqd --lookupd-tcp-addres="127.0.0.1:4160"
```

Setea el path para tu base de datos en otra terminal

```
mongod --dbpath ./db
```

Crea tu propia colección con las palabras claves correspondientes en mongoDB

```
mongo
> use ballots
> db.polls.insert({"title": "someName", "options": {"option1", "option2", "option3"}})
```

Ahora corre los archivos correspondientes en go

```
cd counter/
go run main.go
```

```
cd twittervotes/
go run main.go twitter.go
```
```
cd web/
go run main.go
```
