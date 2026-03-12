#!/bin/bash
# Setup script for email-demo
# Creates the mailserver email account used by the Go app.
#
# Usage:
#   1. Edit docker-compose.yml: set your domain in hostname and OVERRIDE_HOSTNAME
#   2. Run: docker compose up -d mailserver
#   3. Run: bash setup.sh
#   4. Run: docker compose up -d

set -e

EMAIL="${1:-user@example.com}"
PASSWORD="${2:-password}"

echo "Creating email account: $EMAIL"
docker exec mailserver setup email add "$EMAIL" "$PASSWORD"

echo ""
echo "Done! Make sure IMAP_USER and IMAP_PASS in docker-compose.yml match:"
echo "  IMAP_USER=$EMAIL"
echo "  IMAP_PASS=$PASSWORD"
echo ""
echo "Then restart: docker compose up -d"
