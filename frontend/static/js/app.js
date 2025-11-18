// State
let currentUser = null;
let cart = { items: [], total_items: 0, total_price: 0 };
let products = [];

// API Base URL
const API_BASE = '/api';

// Initialize
document.addEventListener('DOMContentLoaded', () => {
    checkAuth();
    loadProducts();
    loadCategories();
});

// Auth Functions
function checkAuth() {
    const token = localStorage.getItem('token');
    const user = localStorage.getItem('user');

    if (token && user) {
        currentUser = JSON.parse(user);
        updateAuthUI(true);
        loadCart();
    } else {
        updateAuthUI(false);
    }
}

function updateAuthUI(isLoggedIn) {
    const authLinks = document.getElementById('auth-links');
    const userLinks = document.getElementById('user-links');
    const userEmail = document.getElementById('user-email');

    if (isLoggedIn) {
        authLinks.style.display = 'none';
        userLinks.style.display = 'flex';
        userEmail.textContent = currentUser.email;
    } else {
        authLinks.style.display = 'flex';
        userLinks.style.display = 'none';
    }
}

async function handleLogin(e) {
    e.preventDefault();

    const email = document.getElementById('login-email').value;
    const password = document.getElementById('login-password').value;

    try {
        const response = await fetch(`${API_BASE}/login`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ email, password })
        });

        if (!response.ok) throw new Error('Invalid credentials');

        const data = await response.json();
        localStorage.setItem('token', data.token);
        localStorage.setItem('user', JSON.stringify(data.user));
        currentUser = data.user;

        updateAuthUI(true);
        loadCart();
        showSection('products');
        showToast('Login successful!', 'success');
    } catch (error) {
        showToast(error.message, 'error');
    }
}

async function handleRegister(e) {
    e.preventDefault();

    const user = {
        email: document.getElementById('register-email').value,
        password: document.getElementById('register-password').value,
        first_name: document.getElementById('register-firstname').value,
        last_name: document.getElementById('register-lastname').value,
        phone: document.getElementById('register-phone').value,
        address: document.getElementById('register-address').value
    };

    try {
        const response = await fetch(`${API_BASE}/register`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(user)
        });

        if (!response.ok) throw new Error('Registration failed');

        const data = await response.json();
        localStorage.setItem('token', data.token);
        localStorage.setItem('user', JSON.stringify(data.user));
        currentUser = data.user;

        updateAuthUI(true);
        showSection('products');
        showToast('Registration successful!', 'success');
    } catch (error) {
        showToast(error.message, 'error');
    }
}

function logout() {
    localStorage.removeItem('token');
    localStorage.removeItem('user');
    currentUser = null;
    cart = { items: [], total_items: 0, total_price: 0 };
    updateAuthUI(false);
    updateCartCount();
    showSection('products');
    showToast('Logged out successfully', 'info');
}

// Products Functions
async function loadProducts() {
    try {
        const response = await fetch(`${API_BASE}/products`);
        products = await response.json();
        renderProducts(products);
    } catch (error) {
        document.getElementById('products-grid').innerHTML =
            '<div class="empty-state"><h3>Failed to load products</h3></div>';
    }
}

async function loadCategories() {
    try {
        const response = await fetch(`${API_BASE}/categories`);
        const categories = await response.json();

        const select = document.getElementById('category-filter');
        categories.forEach(cat => {
            const option = document.createElement('option');
            option.value = cat.name;
            option.textContent = cat.name;
            select.appendChild(option);
        });
    } catch (error) {
        console.error('Failed to load categories');
    }
}

function renderProducts(productsToRender) {
    const grid = document.getElementById('products-grid');

    if (!productsToRender || productsToRender.length === 0) {
        grid.innerHTML = '<div class="empty-state"><h3>No products found</h3></div>';
        return;
    }

    grid.innerHTML = productsToRender.map(product => `
        <div class="product-card">
            <img src="${product.image_url || 'https://via.placeholder.com/300x200?text=' + encodeURIComponent(product.name)}"
                 alt="${product.name}" class="product-image">
            <div class="product-info">
                <div class="product-category">${product.category}</div>
                <h3 class="product-name">${product.name}</h3>
                <div class="product-price">$${product.price.toFixed(2)}</div>
                <div class="product-stock ${product.stock <= 0 ? 'out-of-stock' : ''}">
                    ${product.stock > 0 ? `${product.stock} in stock` : 'Out of stock'}
                </div>
                <button class="btn btn-primary btn-block"
                        onclick="addToCart(${product.id}, '${product.name}', ${product.price}, '${product.image_url || ''}')"
                        ${product.stock <= 0 ? 'disabled' : ''}>
                    Add to Cart
                </button>
            </div>
        </div>
    `).join('');
}

function filterProducts() {
    const category = document.getElementById('category-filter').value;
    const search = document.getElementById('search-input').value.toLowerCase();

    let filtered = products;

    if (category) {
        filtered = filtered.filter(p => p.category === category);
    }

    if (search) {
        filtered = filtered.filter(p =>
            p.name.toLowerCase().includes(search) ||
            p.description.toLowerCase().includes(search)
        );
    }

    renderProducts(filtered);
}

// Cart Functions
async function loadCart() {
    if (!currentUser) return;

    try {
        const response = await fetch(`${API_BASE}/cart/${currentUser.id}`, {
            headers: { 'Authorization': `Bearer ${localStorage.getItem('token')}` }
        });
        cart = await response.json();
        updateCartCount();
    } catch (error) {
        console.error('Failed to load cart');
    }
}

async function addToCart(productId, name, price, imageUrl) {
    if (!currentUser) {
        showToast('Please login to add items to cart', 'error');
        showSection('login');
        return;
    }

    try {
        const response = await fetch(`${API_BASE}/cart/${currentUser.id}/items`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${localStorage.getItem('token')}`
            },
            body: JSON.stringify({
                product_id: productId,
                quantity: 1,
                price: price,
                name: name,
                image_url: imageUrl
            })
        });

        if (!response.ok) throw new Error('Failed to add to cart');

        await loadCart();
        showToast('Added to cart!', 'success');
    } catch (error) {
        showToast(error.message, 'error');
    }
}

async function updateCartItem(itemId, quantity) {
    try {
        const response = await fetch(`${API_BASE}/cart/${currentUser.id}/items/${itemId}`, {
            method: 'PUT',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${localStorage.getItem('token')}`
            },
            body: JSON.stringify({ quantity })
        });

        if (!response.ok) throw new Error('Failed to update cart');

        await loadCart();
        renderCart();
    } catch (error) {
        showToast(error.message, 'error');
    }
}

async function removeFromCart(itemId) {
    try {
        const response = await fetch(`${API_BASE}/cart/${currentUser.id}/items/${itemId}`, {
            method: 'DELETE',
            headers: { 'Authorization': `Bearer ${localStorage.getItem('token')}` }
        });

        if (!response.ok) throw new Error('Failed to remove from cart');

        await loadCart();
        renderCart();
        showToast('Item removed', 'info');
    } catch (error) {
        showToast(error.message, 'error');
    }
}

function updateCartCount() {
    document.getElementById('cart-count').textContent = cart.total_items || 0;
}

function renderCart() {
    const cartItems = document.getElementById('cart-items');
    const cartTotal = document.getElementById('cart-total');

    if (!cart.items || cart.items.length === 0) {
        cartItems.innerHTML = '<div class="empty-state"><h3>Your cart is empty</h3><p>Add some products to get started</p></div>';
        cartTotal.textContent = '$0.00';
        return;
    }

    cartItems.innerHTML = cart.items.map(item => `
        <div class="cart-item">
            <img src="${item.image_url || 'https://via.placeholder.com/80?text=Product'}"
                 alt="${item.name}" class="cart-item-image">
            <div class="cart-item-info">
                <div class="cart-item-name">${item.name}</div>
                <div class="cart-item-price">$${item.price.toFixed(2)}</div>
            </div>
            <div class="cart-item-quantity">
                <button onclick="updateCartItem(${item.id}, ${item.quantity - 1})">-</button>
                <span>${item.quantity}</span>
                <button onclick="updateCartItem(${item.id}, ${item.quantity + 1})">+</button>
            </div>
            <button class="btn btn-danger" onclick="removeFromCart(${item.id})">Remove</button>
        </div>
    `).join('');

    cartTotal.textContent = `$${cart.total_price.toFixed(2)}`;
}

// Checkout Functions
function checkout() {
    if (!currentUser) {
        showToast('Please login to checkout', 'error');
        showSection('login');
        return;
    }

    if (!cart.items || cart.items.length === 0) {
        showToast('Your cart is empty', 'error');
        return;
    }

    renderCheckout();
    showSection('checkout');
}

function renderCheckout() {
    const checkoutItems = document.getElementById('checkout-items');
    const checkoutTotal = document.getElementById('checkout-total');
    const shippingAddress = document.getElementById('shipping-address');

    // Pre-fill shipping address if user has one
    if (currentUser.address) {
        shippingAddress.value = currentUser.address;
    }

    checkoutItems.innerHTML = cart.items.map(item => `
        <div class="checkout-item">
            <span>${item.name} x ${item.quantity}</span>
            <span>$${(item.price * item.quantity).toFixed(2)}</span>
        </div>
    `).join('');

    checkoutTotal.textContent = `$${cart.total_price.toFixed(2)}`;
}

async function handleCheckout(e) {
    e.preventDefault();

    const cardNumber = document.getElementById('card-number').value.replace(/\s/g, '');
    const expiry = document.getElementById('card-expiry').value.split('/');
    const cvc = document.getElementById('card-cvc').value;
    const shippingAddress = document.getElementById('shipping-address').value;

    try {
        // Create order
        const orderResponse = await fetch(`${API_BASE}/orders`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${localStorage.getItem('token')}`
            },
            body: JSON.stringify({
                user_id: currentUser.id,
                total_amount: cart.total_price,
                shipping_address: shippingAddress,
                payment_method: 'card',
                items: cart.items.map(item => ({
                    product_id: item.product_id,
                    name: item.name,
                    quantity: item.quantity,
                    price: item.price
                }))
            })
        });

        if (!orderResponse.ok) throw new Error('Failed to create order');

        const order = await orderResponse.json();

        // Process payment
        const paymentResponse = await fetch(`${API_BASE}/payments`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${localStorage.getItem('token')}`
            },
            body: JSON.stringify({
                order_id: order.id,
                user_id: currentUser.id,
                amount: cart.total_price,
                currency: 'USD',
                method: 'card',
                card_info: {
                    number: cardNumber,
                    exp_month: expiry[0],
                    exp_year: expiry[1],
                    cvc: cvc
                }
            })
        });

        const payment = await paymentResponse.json();

        if (payment.status === 'completed') {
            // Clear cart
            await fetch(`${API_BASE}/cart/${currentUser.id}`, {
                method: 'DELETE',
                headers: { 'Authorization': `Bearer ${localStorage.getItem('token')}` }
            });

            cart = { items: [], total_items: 0, total_price: 0 };
            updateCartCount();

            showToast('Order placed successfully!', 'success');
            showSection('orders');
            loadOrders();
        } else {
            showToast('Payment failed: ' + (payment.error_message || 'Please try again'), 'error');
        }
    } catch (error) {
        showToast(error.message, 'error');
    }
}

// Orders Functions
async function loadOrders() {
    if (!currentUser) return;

    try {
        const response = await fetch(`${API_BASE}/orders/user/${currentUser.id}`, {
            headers: { 'Authorization': `Bearer ${localStorage.getItem('token')}` }
        });
        const orders = await response.json();
        renderOrders(orders);
    } catch (error) {
        document.getElementById('orders-list').innerHTML =
            '<div class="empty-state"><h3>Failed to load orders</h3></div>';
    }
}

function renderOrders(orders) {
    const ordersList = document.getElementById('orders-list');

    if (!orders || orders.length === 0) {
        ordersList.innerHTML = '<div class="empty-state"><h3>No orders yet</h3><p>Start shopping to see your orders here</p></div>';
        return;
    }

    ordersList.innerHTML = orders.map(order => `
        <div class="order-card">
            <div class="order-header">
                <span class="order-id">Order #${order.id}</span>
                <span class="order-status ${order.status}">${order.status}</span>
            </div>
            <div class="order-details">
                <div>
                    <span class="order-detail-label">Date</span>
                    <span>${new Date(order.created_at).toLocaleDateString()}</span>
                </div>
                <div>
                    <span class="order-detail-label">Total</span>
                    <span>$${order.total_amount.toFixed(2)}</span>
                </div>
                <div>
                    <span class="order-detail-label">Payment</span>
                    <span>${order.payment_status}</span>
                </div>
            </div>
        </div>
    `).join('');
}

// UI Functions
function showSection(section) {
    // Hide all sections
    document.querySelectorAll('.section').forEach(el => el.style.display = 'none');

    // Show requested section
    const sectionEl = document.getElementById(`${section}-section`);
    if (sectionEl) {
        sectionEl.style.display = 'block';
    }

    // Load section-specific data
    switch (section) {
        case 'cart':
            renderCart();
            break;
        case 'orders':
            if (currentUser) {
                loadOrders();
            } else {
                showToast('Please login to view orders', 'error');
                showSection('login');
            }
            break;
        case 'products':
            loadProducts();
            break;
    }
}

function showToast(message, type = 'info') {
    const toast = document.getElementById('toast');
    toast.textContent = message;
    toast.className = `toast ${type} show`;

    setTimeout(() => {
        toast.classList.remove('show');
    }, 3000);
}

// API Helper
function getAuthHeaders() {
    const token = localStorage.getItem('token');
    return token ? { 'Authorization': `Bearer ${token}` } : {};
}
