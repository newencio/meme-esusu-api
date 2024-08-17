
# Meme API Service

## Overview

The service supports generating and serving memes via GIPHY based on geographical and text-based queries, and implements an authentication mechanism where clients are charged based on API usage. The service is built using Go and SQLite for simplicity and ease of setup.

## Features

- **Token-Based Authentication**: Clients authenticate using a token provided in the request headers. Tokens are generated through the `/generate_token` endpoint.
- **Meme Generation**: The `/memes` endpoint serves memes based on geographical coordinates and a text query.
- **Automatic Database Setup**: The SQLite database is automatically created and initialized with the necessary tables when the service is first run.

## Requirements

- **Go**: Version 1.15 or higher

## Setup Instructions

### 1. Clone the Repository

```bash
git clone https://github.com/newencio/meme-esusu-api
cd meme-api-service
```

### 2. Install Dependencies

Ensure you have Go installed. Then, install the required Go packages:

```bash
go mod tidy
```

### 3. Run the Service

You can start the service using the following command:

```bash
go run main.go
```

This will start the API service on `http://localhost:8080`.

## API Endpoints

### 1. Generate Auth Token

- **Endpoint**: `/generate_token`
- **Method**: `GET`
- **Description**: Generates a new authentication token and stores it in the database with an initial balance of 100 tokens.

**Example Request**:
```bash
curl http://localhost:8080/generate_token
```

**Example Response**:
```json
{
    "auth_token": "generated-uuid-token-here"
}
```

### 2. Get a Meme

- **Endpoint**: `/memes`
- **Method**: `GET`
- **Headers**: 
  - `Authorization`: The auth token generated from `/generate_token`.
- **Query Parameters**:
  - `lat`: Latitude of the location (e.g., `40.730610`).
  - `lon`: Longitude of the location (e.g., `-73.935242`).
  - `query`: A text-based query (e.g., `food`).

**Example Request**:
```bash
curl -H "Authorization: your-auth-token" "http://localhost:8080/memes?lat=40.730610&lon=-73.935242&query=food"
```

**Example Response**:
```json
{
    "url": "https://example.com/meme.jpg",
    "location": "New York City",
    "query": "food"
}
```

### 3. Update Token Balance

- **Endpoint**: `/update_tokens`
- **Method**: `GET`
- **Query Parameters**:
  - `auth_token`: The auth token of the client.
  - `tokens`: Number of tokens to add to the client's balance.

**Example Request**:
```bash
curl "http://localhost:8080/update_tokens?auth_token=your-auth-token&tokens=50"
```

**Example Response**:
```bash
Tokens updated successfully
```

## Running Tests

Unit tests are provided to ensure the functionality of the API endpoints. To run the tests, use the following command:

```bash
go test -v
```

This will execute all the test cases defined in the `main_test.go` file.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for more details.
