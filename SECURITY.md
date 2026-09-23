# Security Configuration

This document describes the environment variables required for secure operation of the Nexus matching engine.

## Environment Variables

### JWT_SECRET
- **Purpose**: Used for signing and verifying JSON Web Tokens (JWT) for authentication
- **Format**: A strong, random string (32+ characters recommended)
- **Example**: `export JWT_SECRET="your-strong-random-secret-here-with-at-least-32-characters"`
- **Security Note**: The previous hardcoded secret "NexusPassword" has been removed from the codebase

### TWELVEDATA_API_KEY
- **Purpose**: API key for accessing TwelveData financial data services
- **Format**: Your TwelveData API key (alphanumeric string)
- **Example**: `export TWELVEDATA_API_KEY="your-twelvedata-api-key-here"`
- **Security Note**: The previous hardcoded API key "081f90e89a2447a48c79296b458cfd98" has been removed from the codebase

## Setup Instructions

### Option 1: Using Environment Variables (Recommended for Production)
```bash
# Set environment variables in your terminal
export JWT_SECRET="your-generated-secret"
export TWELVEDATA_API_KEY="your-new-api-key"
go run cmd/api/main.go
```

### Option 2: Using .env File (Recommended for Local Development)

1. **Copy the example file**:
   ```bash
   cp .env.example .env
   ```

2. **Edit the .env file** with your actual values:
   ```bash
   # Edit .env file
   nano .env
   ```

3. **Load environment variables** (or use a package like github.com/joho/godotenv):
   ```bash
   # Using source
   source .env
   go run cmd/api/main.go

   # Or install godotenv for automatic loading
   go get github.com/joho/godotenv
   ```

### Generating Secrets

1. **Generate a new JWT secret**:
   ```bash
   # Generate a strong random secret (32+ characters)
   openssl rand -base64 32
   ```

2. **Obtain a new TwelveData API key**:
   - Visit [TwelveData website](https://twelvedata.com/)
   - Sign up for an account or log in
   - Generate a new API key in your account dashboard
   - **Important**: Rotate your existing API key if it was exposed

## Important Security Notes

- **Never commit `.env` files** to version control (they're in `.gitignore`)
- **Use strong secrets**: JWT secrets should be 32+ characters
- **Rotate compromised keys**: If any secrets were exposed, generate new ones immediately
- **Monitor API usage**: Set up alerts for unusual activity on your TwelveData account

## Security Best Practices

1. **Never commit secrets to version control**: Use environment variables or secret management systems
2. **Rotate compromised keys**: If any secrets were exposed, generate new ones immediately
3. **Use strong secrets**: JWT secrets should be long, random strings
4. **Limit API key permissions**: Use API keys with least privilege access
5. **Monitor API usage**: Set up alerts for unusual activity on your TwelveData account

## Running the Application

```bash
# Set environment variables and run
JWT_SECRET="your-secret" TWELVEDATA_API_KEY="your-api-key" go run cmd/api/main.go
```

## Additional Security Notes

- The hardcoded secrets have been removed from the source code
- Environment variables are now used throughout the application
- For complete removal from git history, consider using `git filter-repo` or `BFG` tools if the secrets were previously committed