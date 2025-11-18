-- Create databases for each microservice
CREATE DATABASE users_db;
CREATE DATABASE products_db;
CREATE DATABASE cart_db;
CREATE DATABASE orders_db;
CREATE DATABASE payments_db;
CREATE DATABASE notifications_db;

-- Connect to products_db and seed data
\c products_db;

CREATE TABLE IF NOT EXISTS categories (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) UNIQUE NOT NULL
);

CREATE TABLE IF NOT EXISTS products (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    price DECIMAL(10,2) NOT NULL,
    stock INT DEFAULT 0,
    category VARCHAR(100),
    image_url TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Insert sample categories
INSERT INTO categories (name) VALUES
    ('Electronics'),
    ('Clothing'),
    ('Books'),
    ('Home & Garden'),
    ('Sports')
ON CONFLICT DO NOTHING;

-- Insert sample products
INSERT INTO products (name, description, price, stock, category, image_url) VALUES
    ('Wireless Headphones', 'High-quality Bluetooth headphones with noise cancellation', 149.99, 50, 'Electronics', 'https://images.unsplash.com/photo-1505740420928-5e560c06d30e?w=300'),
    ('Smart Watch', 'Fitness tracker with heart rate monitor and GPS', 299.99, 30, 'Electronics', 'https://images.unsplash.com/photo-1523275335684-37898b6baf30?w=300'),
    ('Laptop Stand', 'Ergonomic aluminum laptop stand for better posture', 49.99, 100, 'Electronics', 'https://images.unsplash.com/photo-1527864550417-7fd91fc51a46?w=300'),
    ('Cotton T-Shirt', 'Comfortable 100% organic cotton t-shirt', 29.99, 200, 'Clothing', 'https://images.unsplash.com/photo-1521572163474-6864f9cf17ab?w=300'),
    ('Denim Jeans', 'Classic fit denim jeans with stretch', 79.99, 150, 'Clothing', 'https://images.unsplash.com/photo-1542272604-787c3835535d?w=300'),
    ('Running Shoes', 'Lightweight running shoes with cushioned sole', 119.99, 80, 'Sports', 'https://images.unsplash.com/photo-1542291026-7eec264c27ff?w=300'),
    ('Yoga Mat', 'Non-slip yoga mat with carrying strap', 39.99, 120, 'Sports', 'https://images.unsplash.com/photo-1601925260368-ae2f83cf8b7f?w=300'),
    ('Programming Book', 'Learn Go programming from scratch', 44.99, 60, 'Books', 'https://images.unsplash.com/photo-1532012197267-da84d127e765?w=300'),
    ('Coffee Maker', 'Automatic drip coffee maker with timer', 89.99, 40, 'Home & Garden', 'https://images.unsplash.com/photo-1495474472287-4d71bcdd2085?w=300'),
    ('Plant Pot Set', 'Set of 3 ceramic plant pots with drainage', 34.99, 90, 'Home & Garden', 'https://images.unsplash.com/photo-1485955900006-10f4d324d411?w=300'),
    ('Bluetooth Speaker', 'Portable waterproof speaker with 20hr battery', 79.99, 70, 'Electronics', 'https://images.unsplash.com/photo-1608043152269-423dbba4e7e1?w=300'),
    ('Winter Jacket', 'Insulated waterproof winter jacket', 199.99, 45, 'Clothing', 'https://images.unsplash.com/photo-1544923246-77307dd628cb?w=300');
