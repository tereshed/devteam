#!/bin/bash

# Скрипт для тестирования API endpoints
# Использование: ./scripts/test_endpoints.sh [base_url]
# Пример: ./scripts/test_endpoints.sh http://localhost:8080

BASE_URL="${1:-http://localhost:8080}"
API_URL="${BASE_URL}/api/v1"

echo "🧪 Тестирование API endpoints на ${BASE_URL}"
echo "=========================================="
echo ""

# Цвета для вывода
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Функция для проверки ответа
check_response() {
    local name=$1
    local status=$2
    local expected_status=$3
    
    if [ "$status" -eq "$expected_status" ]; then
        echo -e "${GREEN}✓${NC} $name - Status: $status"
        return 0
    else
        echo -e "${RED}✗${NC} $name - Expected: $expected_status, Got: $status"
        return 1
    fi
}

# Проверка health check
echo "1. Health Check"
response=$(curl -s -w "\n%{http_code}" "${BASE_URL}/health")
status=$(echo "$response" | tail -n1)
body=$(echo "$response" | sed '$d')
check_response "Health Check" "$status" "200"
echo "   Response: $body"
echo ""

# Регистрация пользователя
echo "2. Регистрация пользователя"
register_response=$(curl -s -w "\n%{http_code}" -X POST "${API_URL}/auth/register" \
    -H "Content-Type: application/json" \
    -d '{
        "email": "test@example.com",
        "password": "password123"
    }')
register_status=$(echo "$register_response" | tail -n1)
register_body=$(echo "$register_response" | sed '$d')
check_response "Register" "$register_status" "201"

if [ "$register_status" -eq "201" ]; then
    ACCESS_TOKEN=$(echo "$register_body" | grep -o '"access_token":"[^"]*' | cut -d'"' -f4)
    REFRESH_TOKEN=$(echo "$register_body" | grep -o '"refresh_token":"[^"]*' | cut -d'"' -f4)
    echo "   Access Token получен: ${ACCESS_TOKEN:0:20}..."
    echo "   Refresh Token получен: ${REFRESH_TOKEN:0:20}..."
else
    echo "   Response: $register_body"
fi
echo ""

# Попытка повторной регистрации (должна вернуть 409)
echo "3. Попытка повторной регистрации (ожидается ошибка)"
duplicate_response=$(curl -s -w "\n%{http_code}" -X POST "${API_URL}/auth/register" \
    -H "Content-Type: application/json" \
    -d '{
        "email": "test@example.com",
        "password": "password123"
    }')
duplicate_status=$(echo "$duplicate_response" | tail -n1)
check_response "Duplicate Register" "$duplicate_status" "409"
echo ""

# Вход
echo "4. Вход пользователя"
login_response=$(curl -s -w "\n%{http_code}" -X POST "${API_URL}/auth/login" \
    -H "Content-Type: application/json" \
    -d '{
        "email": "test@example.com",
        "password": "password123"
    }')
login_status=$(echo "$login_response" | tail -n1)
login_body=$(echo "$login_response" | sed '$d')
check_response "Login" "$login_status" "200"

if [ "$login_status" -eq "200" ]; then
    ACCESS_TOKEN=$(echo "$login_body" | grep -o '"access_token":"[^"]*' | cut -d'"' -f4)
    REFRESH_TOKEN=$(echo "$login_body" | grep -o '"refresh_token":"[^"]*' | cut -d'"' -f4)
    echo "   Access Token получен: ${ACCESS_TOKEN:0:20}..."
    echo "   Refresh Token получен: ${REFRESH_TOKEN:0:20}..."
fi
echo ""

# Неверные учетные данные
echo "5. Вход с неверными учетными данными"
invalid_login_response=$(curl -s -w "\n%{http_code}" -X POST "${API_URL}/auth/login" \
    -H "Content-Type: application/json" \
    -d '{
        "email": "test@example.com",
        "password": "wrongpassword"
    }')
invalid_login_status=$(echo "$invalid_login_response" | tail -n1)
check_response "Invalid Login" "$invalid_login_status" "401"
echo ""

# Обновление токена
if [ -n "$REFRESH_TOKEN" ]; then
    echo "6. Обновление токена"
    refresh_response=$(curl -s -w "\n%{http_code}" -X POST "${API_URL}/auth/refresh" \
        -H "Content-Type: application/json" \
        -d "{
            \"refresh_token\": \"$REFRESH_TOKEN\"
        }")
    refresh_status=$(echo "$refresh_response" | tail -n1)
    refresh_body=$(echo "$refresh_response" | sed '$d')
    check_response "Refresh Token" "$refresh_status" "200"
    
    if [ "$refresh_status" -eq "200" ]; then
        NEW_ACCESS_TOKEN=$(echo "$refresh_body" | grep -o '"access_token":"[^"]*' | cut -d'"' -f4)
        NEW_REFRESH_TOKEN=$(echo "$refresh_body" | grep -o '"refresh_token":"[^"]*' | cut -d'"' -f4)
        echo "   Новый Access Token получен: ${NEW_ACCESS_TOKEN:0:20}..."
        ACCESS_TOKEN=$NEW_ACCESS_TOKEN
    fi
    echo ""
fi

# Выход (требует авторизации)
if [ -n "$ACCESS_TOKEN" ]; then
    echo "7. Выход пользователя"
    logout_response=$(curl -s -w "\n%{http_code}" -X POST "${API_URL}/auth/logout" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $ACCESS_TOKEN")
    logout_status=$(echo "$logout_response" | tail -n1)
    logout_body=$(echo "$logout_response" | sed '$d')
    check_response "Logout" "$logout_status" "200"
    echo "   Response: $logout_body"
    echo ""
fi

# Выход без токена (должен вернуть 401)
echo "8. Выход без токена (ожидается ошибка)"
no_auth_logout=$(curl -s -w "\n%{http_code}" -X POST "${API_URL}/auth/logout" \
    -H "Content-Type: application/json")
no_auth_status=$(echo "$no_auth_logout" | tail -n1)
check_response "Logout without Auth" "$no_auth_status" "401"
echo ""

echo "=========================================="
echo -e "${GREEN}Тестирование завершено!${NC}"

