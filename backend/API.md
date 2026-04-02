# API Documentation

## Authentication Endpoints

### POST /api/v1/auth/register

Регистрация нового пользователя.

**Request Body:**
```json
{
  "email": "user@example.com",
  "password": "password123"
}
```

**Response (201):**
```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refresh_token": "a1b2c3d4e5f6...",
  "token_type": "Bearer",
  "expires_in": 900
}
```

**Errors:**
- `400` - Невалидные данные
- `409` - Пользователь уже существует

---

### POST /api/v1/auth/login

Вход пользователя.

**Request Body:**
```json
{
  "email": "user@example.com",
  "password": "password123"
}
```

**Response (200):**
```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refresh_token": "a1b2c3d4e5f6...",
  "token_type": "Bearer",
  "expires_in": 900
}
```

**Errors:**
- `400` - Невалидные данные
- `401` - Неверные учетные данные

---

### POST /api/v1/auth/refresh

Обновление access token используя refresh token.

**Request Body:**
```json
{
  "refresh_token": "a1b2c3d4e5f6..."
}
```

**Response (200):**
```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refresh_token": "new_refresh_token...",
  "token_type": "Bearer",
  "expires_in": 900
}
```

**Errors:**
- `400` - Невалидные данные
- `401` - Невалидный refresh token

---

### POST /api/v1/auth/logout

Выход пользователя (отзывает все refresh токены).

**Headers:**
```
Authorization: Bearer <access_token>
```

**Response (200):**
```json
{
  "message": "logged out successfully"
}
```

**Errors:**
- `401` - Не авторизован

---

### GET /api/v1/auth/me

Получение данных текущего аутентифицированного пользователя.

**Headers:**
```
Authorization: Bearer <access_token>
```

**Response (200):**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "email": "user@example.com",
  "role": "user",
  "email_verified": false
}
```

**Errors:**
- `401` - Не авторизован

---

## Health Check

### GET /health

Проверка состояния сервера и базы данных.

**Response (200):**
```json
{
  "status": "healthy",
  "timestamp": 1234567890
}
```

