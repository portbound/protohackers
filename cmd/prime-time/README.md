# Prime Time

## Requirements
- [x] accept TCP conns
- [x] receive Conforming Request == send Conforming Response and wait for another request
- [x] receive Malformed Request == send single Malformed Response and **disconnect** 
- [x] handle at least 5 clients
- [x] each request should be handled in order

## Tips
- each request is a single line of JSON 
- a client may send multiple requests before disconnecting 
- non-integers can not be prime

## Requests
### Conforming 
- field "method" = "isPrime"
- field "number" = JSON Number 
e.g.
=={"method":"isPrime","number":123}==

### Malformed
- broken JSON format
- any field is missing 
- method name is not "isPrime"
- number value is not a number 
- extraneous fields should be ignored

## Response
### Conforming 
- field "method" = "isPrime"
- field "prime" = bool
e.g.
=={"method":"isPrime","prime":false}==

### Malformed
- broken JSON format
- any field is missing
- method name is not "isPrime"
- prime value is not a bool

