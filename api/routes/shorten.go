package routes

import (
	"os"
	"strconv"
	"time"
	"log"
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

    if err := c.BodyParser(&body); err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "cannot parse JSON"})
    }

    // Implement rate limiting
    rdb2 := database.CreateClient(1)
    defer rdb2.Close()

    apiQuota, err := strconv.Atoi(os.Getenv("API_QUOTA"))
    if err != nil {
        log.Println("Invalid API_QUOTA value in environment variables")
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "server configuration error"})
    }

    value, err := rdb2.Get(database.Ctx, c.IP()).Result()
    if err == redis.Nil {
        log.Println("Key does not exist, initializing rate limit")
        err = rdb2.Set(database.Ctx, c.IP(), apiQuota, QUOTA_RESET).Err()
        if err != nil {
            log.Println("Failed to set rate limit:", err)
            return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "server error"})
        }
        value = strconv.Itoa(apiQuota)
    } else if err != nil {
        log.Println("Failed to get rate limit:", err)
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "server error"})
    }

    quotaRemaining, _ := strconv.Atoi(value)
    if quotaRemaining <= 0 {
        limit, err := rdb2.TTL(database.Ctx, c.IP()).Result()
        if err != nil {
            log.Println("Failed to get TTL:", err)
            return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "server error"})
        }
        return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
            "error":            "rate limit exceeded",
            "rate_limit_reset": limit / time.Nanosecond / time.Minute,
        })
    }

    // Check if the input is an actual URL
    if !govalidator.IsURL(body.URL) {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "not a valid URL"})
    }

    // Check for domain error
    if !helpers.RemoveDomainError(body.URL) {
        return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "infinite loop :p"})
    }

    // Enforce HTTPS, SSL
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
            "error": "the custom short is already taken!",
        })
    }

    if body.Expiry == 0 {
        body.Expiry = 24 * time.Hour
    }

    err = rdb.Set(database.Ctx, id, body.URL, body.Expiry).Err()
    if err != nil {
        log.Println("Failed to set URL in Redis:", err)
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "error": "unable to connect to the server",
        })
    }

    // Decrement the rate limit
    err = rdb2.Decr(database.Ctx, c.IP()).Err()
    if err != nil {
        log.Println("Failed to decrement rate limit:", err)
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "server error"})
    }

    // Prepare the response
    resp := response{
        URL:             body.URL,
        CustomShort:     os.Getenv("DOMAIN") + "/" + id,
        Expiry:          body.Expiry,
        XRateRemaining:  quotaRemaining - 1,
        XRateLimitReset: 0,
    }

    ttl, err := rdb2.TTL(database.Ctx, c.IP()).Result()
    if err == nil {
        resp.XRateLimitReset = ttl / time.Nanosecond / time.Minute
    }

    return c.Status(fiber.StatusOK).JSON(resp)
}