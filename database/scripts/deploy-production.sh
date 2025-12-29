#!/bin/bash
# SheetMaster Production Deployment Script
# Bu script production ga deploy qilishni osonlashtiradi

set -e

echo "=========================================="
echo "  SheetMaster Production Deployment"
echo "=========================================="
echo ""

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check if running as root
if [ "$EUID" -eq 0 ]; then
    echo -e "${RED}Error: Do not run this script as root!${NC}"
    exit 1
fi

# Check if Docker is installed
if ! command -v docker &> /dev/null; then
    echo -e "${RED}Error: Docker is not installed!${NC}"
    echo "Install Docker first: sudo apt install -y docker.io"
    exit 1
fi

# Check if Docker Compose is installed
if ! command -v docker-compose &> /dev/null; then
    echo -e "${RED}Error: Docker Compose is not installed!${NC}"
    echo "Install Docker Compose: sudo apt install -y docker-compose-v2"
    exit 1
fi

# Check if .env.production exists
if [ ! -f .env.production ]; then
    echo -e "${YELLOW}Warning: .env.production not found!${NC}"
    echo "Creating from example..."

    if [ -f .env.production.example ]; then
        cp .env.production.example .env.production
        echo -e "${GREEN}✓ Created .env.production${NC}"
        echo ""
        echo -e "${YELLOW}IMPORTANT: Edit .env.production and set your values!${NC}"
        echo "Run: nano .env.production"
        echo ""
        read -p "Press Enter after editing .env.production..."
    else
        echo -e "${RED}Error: .env.production.example not found!${NC}"
        exit 1
    fi
fi

# Load environment variables
set -a
source .env.production
set +a

echo "Loaded environment variables from .env.production"
echo ""

# Validate required variables
REQUIRED_VARS=(
    "DB_USER"
    "DB_PASSWORD"
    "DB_NAME"
    "JWT_SECRET"
    "ELIXIR_SECRET_KEY_BASE"
    "REDIS_PASSWORD"
    "DOMAIN"
)

echo "Validating environment variables..."
MISSING_VARS=()

for var in "${REQUIRED_VARS[@]}"; do
    if [ -z "${!var}" ]; then
        MISSING_VARS+=("$var")
    fi
done

if [ ${#MISSING_VARS[@]} -ne 0 ]; then
    echo -e "${RED}Error: Missing required environment variables:${NC}"
    for var in "${MISSING_VARS[@]}"; do
        echo "  - $var"
    done
    echo ""
    echo "Edit .env.production and set these variables"
    exit 1
fi

echo -e "${GREEN}✓ All required variables are set${NC}"
echo ""

# Check SSL certificates
echo "Checking SSL certificates..."
if [ ! -f "nginx/ssl/fullchain.pem" ] || [ ! -f "nginx/ssl/privkey.pem" ]; then
    echo -e "${YELLOW}Warning: SSL certificates not found!${NC}"
    echo ""
    echo "You need to obtain SSL certificates first."
    echo "Run: sudo certbot certonly --standalone -d $DOMAIN"
    echo ""
    read -p "Continue without SSL? (not recommended for production) [y/N]: " -n 1 -r
    echo ""
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
else
    echo -e "${GREEN}✓ SSL certificates found${NC}"
fi
echo ""

# Create necessary directories
echo "Creating directories..."
mkdir -p backups
mkdir -p nginx/ssl
echo -e "${GREEN}✓ Directories created${NC}"
echo ""

# Build Docker images
echo "=========================================="
echo "Building Docker images..."
echo "=========================================="
docker-compose -f docker-compose.prod.yml build

if [ $? -ne 0 ]; then
    echo -e "${RED}Error: Docker build failed!${NC}"
    exit 1
fi

echo -e "${GREEN}✓ Docker images built successfully${NC}"
echo ""

# Start services
echo "=========================================="
echo "Starting services..."
echo "=========================================="
docker-compose -f docker-compose.prod.yml up -d

if [ $? -ne 0 ]; then
    echo -e "${RED}Error: Failed to start services!${NC}"
    exit 1
fi

echo -e "${GREEN}✓ Services started successfully${NC}"
echo ""

# Wait for services to be healthy
echo "Waiting for services to be ready..."
sleep 10

# Check health endpoints
echo ""
echo "Checking service health..."

# Check Go backend
if curl -f http://localhost:8080/health &> /dev/null; then
    echo -e "${GREEN}✓ Backend API is healthy${NC}"
else
    echo -e "${YELLOW}⚠ Backend API is not responding yet${NC}"
fi

# Check frontend
if curl -f http://localhost/health &> /dev/null; then
    echo -e "${GREEN}✓ Frontend is healthy${NC}"
else
    echo -e "${YELLOW}⚠ Frontend is not responding yet${NC}"
fi

echo ""
echo "=========================================="
echo -e "${GREEN}Deployment completed!${NC}"
echo "=========================================="
echo ""
echo "Your application is now running!"
echo ""
echo "URLs:"
echo "  Frontend: https://$DOMAIN"
echo "  API:      https://api.$DOMAIN"
echo "  Realtime: wss://realtime.$DOMAIN/socket"
echo ""
echo "Useful commands:"
echo "  View logs:    docker-compose -f docker-compose.prod.yml logs -f"
echo "  Check status: docker-compose -f docker-compose.prod.yml ps"
echo "  Stop:         docker-compose -f docker-compose.prod.yml down"
echo "  Restart:      docker-compose -f docker-compose.prod.yml restart"
echo ""
echo "Next steps:"
echo "  1. Configure DNS records to point to this server"
echo "  2. Set up SSL certificates (if not done yet)"
echo "  3. Configure email SMTP settings in .env.production"
echo "  4. Test all functionality"
echo ""
