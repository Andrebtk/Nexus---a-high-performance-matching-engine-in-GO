import { useState, useEffect } from 'react';
import { useAuth } from '../context/AuthContext';
import { useNavigate } from 'react-router-dom';

const API_URL = "http://localhost:8080";

// "TradingView / Binance" color palette
const theme = {
  bg: '#0b0e11',
  panel: '#181a20',
  border: '#2b3139',
  textMain: '#EAECEF',
  textMuted: '#848E9C',
  buy: '#0ecb81',
  sell: '#f6465d',
  accent: '#2962ff'
};

function ProfilePage() {
  const { user } = useAuth();
  const navigate = useNavigate();
  const [activeOrders, setActiveOrders] = useState([]);
  const [orderHistory, setOrderHistory] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  useEffect(() => {
    if (!user || !user.id) {
      navigate('/');
      return;
    }

    const fetchOrders = async () => {
      try {
        setLoading(true);
        setError(null);

        console.log(`DEBUG: Fetching orders for user ${user.id}`);

        // Fetch active orders
        const activeResponse = await fetch(`${API_URL}/orders/active?user_id=${user.id}`);
        console.log(`DEBUG: Active orders response status: ${activeResponse.status}`);
        if (activeResponse.ok) {
          const activeData = await activeResponse.json();
          console.log(`DEBUG: Received ${activeData.active_orders?.length || 0} active orders`);
          setActiveOrders(activeData.active_orders || []);
        }

        // Fetch order history
        const historyResponse = await fetch(`${API_URL}/orders/history?user_id=${user.id}`);
        console.log(`DEBUG: History orders response status: ${historyResponse.status}`);
        if (historyResponse.ok) {
          const historyData = await historyResponse.json();
          console.log(`DEBUG: Received ${historyData.order_history?.length || 0} historical orders`);
          setOrderHistory(historyData.order_history || []);
        }

      } catch (err) {
        console.error("Failed to fetch orders:", err);
        setError("Failed to load order data");
      } finally {
        setLoading(false);
      }
    };

    fetchOrders();
  }, [user, navigate]);

  const formatDate = (dateString) => {
    const date = new Date(dateString);
    return date.toLocaleString();
  };

  const getOrderTypeColor = (orderType) => {
    return orderType === 'BUY' ? theme.buy : theme.sell;
  };

  const getStatusColor = (status) => {
    switch (status) {
      case 'active': return '#FFA500'; // Orange
      case 'completed': return theme.buy; // Green
      case 'cancelled': return theme.sell; // Red
      default: return theme.textMuted;
    }
  };

  if (!user) {
    return <div>Loading...</div>;
  }

  return (
    <div style={{
      minHeight: '100vh',
      backgroundColor: theme.bg,
      color: theme.textMain,
      padding: '20px',
      fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif'
    }}>
      <div style={{ maxWidth: '1200px', margin: '0 auto' }}>
        {/* Header */}
        <div style={{ marginBottom: '20px' }}>
          <h1 style={{
            margin: 0,
            fontSize: '32px',
            fontWeight: '800',
            letterSpacing: '-0.5px',
            marginBottom: '10px'
          }}>
            👤 Profile
          </h1>

          {/* User Info Card */}
          <div style={{
            backgroundColor: theme.panel,
            border: `1px solid ${theme.border}`,
            borderRadius: '8px',
            padding: '20px',
            marginBottom: '20px'
          }}>
            <div style={{ display: 'flex', gap: '20px', alignItems: 'center' }}>
              <div style={{
                width: '64px',
                height: '64px',
                backgroundColor: theme.accent,
                borderRadius: '50%',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                color: 'white',
                fontWeight: 'bold',
                fontSize: '24px'
              }}>
                {user.username.charAt(0).toUpperCase()}
              </div>

              <div style={{ flex: 1 }}>
                <h2 style={{
                  margin: 0,
                  fontSize: '24px',
                  fontWeight: '600',
                  marginBottom: '5px'
                }}>
                  {user.username}
                </h2>
                <div style={{
                  color: theme.textMuted,
                  fontSize: '14px',
                  marginBottom: '10px'
                }}>
                  {user.email}
                </div>

                <div style={{ display: 'flex', gap: '20px' }}>
                  <div>
                    <div style={{ fontSize: '12px', color: theme.textMuted, marginBottom: '5px' }}>
                      Account Balance
                    </div>
                    <div style={{
                      fontSize: '20px',
                      fontWeight: 'bold',
                      color: user.balance >= 0 ? theme.buy : theme.sell
                    }}>
                      ${user.balance?.toFixed(2) || '0.00'}
                    </div>
                  </div>

                  <div>
                    <div style={{ fontSize: '12px', color: theme.textMuted, marginBottom: '5px' }}>
                      Member Since
                    </div>
                    <div style={{ fontSize: '14px', fontWeight: '500' }}>
                      {new Date(user.created_at).toLocaleDateString()}
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        {/* Active Orders Section */}
        <div style={{
          backgroundColor: theme.panel,
          border: `1px solid ${theme.border}`,
          borderRadius: '8px',
          padding: '20px',
          marginBottom: '20px'
        }}>
          <h2 style={{
            marginTop: 0,
            fontSize: '20px',
            fontWeight: '600',
            marginBottom: '15px',
            borderBottom: `1px solid ${theme.border}`,
            paddingBottom: '10px'
          }}>
            🕒 Active Orders
          </h2>

          {loading ? (
            <div style={{ color: theme.textMuted }}>Loading orders...</div>
          ) : error ? (
            <div style={{ color: theme.sell }}>{error}</div>
          ) : activeOrders.length === 0 ? (
            <div style={{ color: theme.textMuted, textAlign: 'center', padding: '20px 0' }}>
              No active orders. Place your first order on the trading page!
            </div>
          ) : (
            <div style={{ overflowX: 'auto' }}>
              <table style={{ width: '100%', borderCollapse: 'collapse' }}>
                <thead>
                  <tr style={{ borderBottom: `1px solid ${theme.border}` }}>
                    <th style={{ textAlign: 'left', padding: '12px', color: theme.textMuted, fontSize: '12px', textTransform: 'uppercase' }}>Symbol</th>
                    <th style={{ textAlign: 'left', padding: '12px', color: theme.textMuted, fontSize: '12px', textTransform: 'uppercase' }}>Type</th>
                    <th style={{ textAlign: 'right', padding: '12px', color: theme.textMuted, fontSize: '12px', textTransform: 'uppercase' }}>Quantity</th>
                    <th style={{ textAlign: 'right', padding: '12px', color: theme.textMuted, fontSize: '12px', textTransform: 'uppercase' }}>Price</th>
                    <th style={{ textAlign: 'right', padding: '12px', color: theme.textMuted, fontSize: '12px', textTransform: 'uppercase' }}>Total</th>
                    <th style={{ textAlign: 'left', padding: '12px', color: theme.textMuted, fontSize: '12px', textTransform: 'uppercase' }}>Created</th>
                    <th style={{ textAlign: 'left', padding: '12px', color: theme.textMuted, fontSize: '12px', textTransform: 'uppercase' }}>Status</th>
                  </tr>
                </thead>
                <tbody>
                  {activeOrders.map((order) => (
                    <tr key={order.id} style={{ borderBottom: `1px solid ${theme.border}` }}>
                      <td style={{ padding: '12px' }}>{order.symbol}</td>
                      <td style={{
                        padding: '12px',
                        color: getOrderTypeColor(order.order_type),
                        fontWeight: '600'
                      }}>
                        {order.order_type}
                      </td>
                      <td style={{ padding: '12px', textAlign: 'right' }}>{order.quantity}</td>
                      <td style={{ padding: '12px', textAlign: 'right' }}>${order.price.toFixed(2)}</td>
                      <td style={{ padding: '12px', textAlign: 'right', fontWeight: '600' }}>
                        ${(order.quantity * order.price).toFixed(2)}
                      </td>
                      <td style={{ padding: '12px', color: theme.textMuted, fontSize: '12px' }}>
                        {formatDate(order.created_at)}
                      </td>
                      <td style={{
                        padding: '12px',
                        color: getStatusColor(order.status),
                        fontWeight: '600'
                      }}>
                        {order.status.toUpperCase()}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>

        {/* Order History Section */}
        <div style={{
          backgroundColor: theme.panel,
          border: `1px solid ${theme.border}`,
          borderRadius: '8px',
          padding: '20px',
          marginBottom: '20px'
        }}>
          <h2 style={{
            marginTop: 0,
            fontSize: '20px',
            fontWeight: '600',
            marginBottom: '15px',
            borderBottom: `1px solid ${theme.border}`,
            paddingBottom: '10px'
          }}>
            📜 Order History
          </h2>

          {loading ? (
            <div style={{ color: theme.textMuted }}>Loading history...</div>
          ) : error ? (
            <div style={{ color: theme.sell }}>{error}</div>
          ) : orderHistory.length === 0 ? (
            <div style={{ color: theme.textMuted, textAlign: 'center', padding: '20px 0' }}>
              No order history yet. Your completed orders will appear here.
            </div>
          ) : (
            <div style={{ overflowX: 'auto' }}>
              <table style={{ width: '100%', borderCollapse: 'collapse' }}>
                <thead>
                  <tr style={{ borderBottom: `1px solid ${theme.border}` }}>
                    <th style={{ textAlign: 'left', padding: '12px', color: theme.textMuted, fontSize: '12px', textTransform: 'uppercase' }}>Symbol</th>
                    <th style={{ textAlign: 'left', padding: '12px', color: theme.textMuted, fontSize: '12px', textTransform: 'uppercase' }}>Type</th>
                    <th style={{ textAlign: 'right', padding: '12px', color: theme.textMuted, fontSize: '12px', textTransform: 'uppercase' }}>Quantity</th>
                    <th style={{ textAlign: 'right', padding: '12px', color: theme.textMuted, fontSize: '12px', textTransform: 'uppercase' }}>Price</th>
                    <th style={{ textAlign: 'right', padding: '12px', color: theme.textMuted, fontSize: '12px', textTransform: 'uppercase' }}>Total</th>
                    <th style={{ textAlign: 'left', padding: '12px', color: theme.textMuted, fontSize: '12px', textTransform: 'uppercase' }}>Completed</th>
                    <th style={{ textAlign: 'left', padding: '12px', color: theme.textMuted, fontSize: '12px', textTransform: 'uppercase' }}>Status</th>
                  </tr>
                </thead>
                <tbody>
                  {orderHistory.map((order) => (
                    <tr key={order.id} style={{ borderBottom: `1px solid ${theme.border}` }}>
                      <td style={{ padding: '12px' }}>{order.symbol}</td>
                      <td style={{
                        padding: '12px',
                        color: getOrderTypeColor(order.order_type),
                        fontWeight: '600'
                      }}>
                        {order.order_type}
                      </td>
                      <td style={{ padding: '12px', textAlign: 'right' }}>{order.quantity}</td>
                      <td style={{ padding: '12px', textAlign: 'right' }}>${order.price.toFixed(2)}</td>
                      <td style={{ padding: '12px', textAlign: 'right', fontWeight: '600' }}>
                        ${(order.quantity * order.price).toFixed(2)}
                      </td>
                      <td style={{ padding: '12px', color: theme.textMuted, fontSize: '12px' }}>
                        {formatDate(order.updated_at)}
                      </td>
                      <td style={{
                        padding: '12px',
                        color: getStatusColor(order.status),
                        fontWeight: '600'
                      }}>
                        {order.status.toUpperCase()}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>

        {/* Back to Trading Button */}
        <div style={{ textAlign: 'center', marginTop: '20px' }}>
          <button
            onClick={() => navigate('/')}
            style={{
              padding: '12px 24px',
              backgroundColor: theme.buy,
              color: 'white',
              border: 'none',
              borderRadius: '6px',
              cursor: 'pointer',
              fontSize: '16px',
              fontWeight: '600',
              transition: 'opacity 0.2s'
            }}
            onMouseOver={(e) => e.target.style.opacity = 0.8}
            onMouseOut={(e) => e.target.style.opacity = 1}
          >
            ← Back to Trading
          </button>
        </div>
      </div>
    </div>
  );
}

export default ProfilePage;