# golinks
go url shortner

Setup:
create a .env file with the following environment variables
1. App port : like ":3000"
2. Address of the database: like "localhost:6379" or "db:6379"
3. DOMAIN: the domain on which the service is running
4. Quota limit an integer
```
APP_PORT = "" 
DB_ADDR = ""
DB_PASS = ""
DOMAIN = ""
API_QUOTA = 
```

Sample usage:
```
$ curl -X POST http://localhost:3000/api/v1 -H "Content-Type: application/json" -d '{
  "URL": "https://example.com",
  "CustomShort": "custom123",
  "Expiry": 24
}'
{"url":"https://example.com","short":"localhost:3000/124b0f","expiry":24,"rate_limit":9,"rate_limit_reset":30} 
```