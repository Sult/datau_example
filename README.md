How to build:

`go build && ./proxyu_client -proxyu "ice2heart.com:9365" -tls-ca-cert tls/root.crt -tls-cert tls/ya.pem -tls-key tls/ya.key`

`npm start`

and glue web server
`devd /=http://localhost:3000 /api/=http://localhost:8090/api `

You also need a proxyu instance. 
