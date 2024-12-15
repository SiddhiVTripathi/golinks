package routes

import (
	"os"
	"strconv"
	"time"

	"github.com/SiddhiVTripathi/golinks/api/database"
	"github.com/SiddhiVTripathi/golinks/api/helpers"
	"github.com/asaskevich/govalidator"
	"github.com/go-redis/redis/v8"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// Define a structure for request
type request struct{
	URL           string			`json:"url"`
	CustomShort	  string			`json:"short"`
	Expiry        time.Duration		`json:"expiry"`
}


// Define a structure for response
type response struct{
	URL				string			`json:"url"`	
	CustomShort		string			`json:"short"`
	Expiry			time.Duration	`json:"expiry"`
	XRateRemaining	int				`json:"rate_limit"`
	XRateLimitReset	time.Duration	`json:"rate_limit_reset"`
}

const QUOTA_RESET = 30*time.Minute

func ShortenURL(c *fiber.Ctx) error {

	body := new(request)

	if err := c.BodyParser(&body); err!=nil{
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error":"cannot parse JSON"})
	}

	//  implement rate limiting
	rdb2 := database.CreateClient(1)
	defer rdb2.Close()

	value, err := rdb2.Get(database.Ctx, c.IP()).Result()

	if err == redis.Nil{
		_ = rdb2.Set(database.Ctx, c.IP(), os.Getenv("API_QUOTA"), QUOTA_RESET).Err()
	} else {
		quotaRemaining, _ := strconv.Atoi(value)
		if quotaRemaining <= 0 {
			limit, _ := rdb2.TTL(database.Ctx, c.IP()).Result()
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"error": "rate limit exceeded",
				"rate_limit_reset" : limit / time.Nanosecond / time.Minute,
			})
		} 
	}

	// check if the input is an actual URL
	if !govalidator.IsURL(body.URL){
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error":"not a valid url"})
	}

	// check for domain error
	if !helpers.RemoveDomainError(body.URL){
		c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error":"infinite loop :p"})
	}

	// enforce https, SSL
	body.URL = helpers.EnforceHTTP(body.URL)

	var id string

	if body.CustomShort == "" {
		id = uuid.New().String()[:6]
	} else {
		id = body.CustomShort
	}

	rdb := database.CreateClient(0)
	defer rdb.Close()

	val, _ := rdb.Get(database.Ctx, id).Result()
	if val != "" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error" : "the custom short is already taken!",
		})
	}

	if body.Expiry == 0 {
		body.Expiry = 24
	}

	err = rdb.Set(database.Ctx, id, body.URL, body.Expiry * time.Hour).Err()

	if err!=nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":"unable to connect to the server",
		})
	}

	resp := response{
		URL: body.URL,
		CustomShort: "",
		Expiry: body.Expiry,
		XRateRemaining: 10,
		XRateLimitReset: 30,
	}

	rdb2.Decr(database.Ctx, c.IP())

	val, _ = rdb2.Get(database.Ctx, c.IP()).Result()
	resp.XRateRemaining, _ = strconv.Atoi(val)

	ttl, _ := rdb2.TTL(database.Ctx, c.IP()).Result()
	resp.XRateLimitReset = ttl / time.Nanosecond / time.Minute

	resp.CustomShort = os.Getenv("DOMAIN") + "/" + id

	return c.Status(fiber.StatusOK).JSON(resp)
}
