# golinks
go url shortner

Sample usage:
```
$ curl -X POST http://localhost:3000/api/v1 -H "Content-Type: application/json" -d '{
  "URL": "https://example.com",
  "CustomShort": "custom123",
  "Expiry": 24
}'
{"url":"https://example.com","short":"localhost:3000/124b0f","expiry":24,"rate_limit":9,"rate_limit_reset":30} 
```