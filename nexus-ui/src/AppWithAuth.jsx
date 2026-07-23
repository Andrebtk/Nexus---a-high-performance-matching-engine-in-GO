import { useState, useEffect } from 'react'
import { LineChart, Line, XAxis, YAxis, Tooltip, ResponsiveContainer, CartesianGrid } from 'recharts'
import { useAuth } from './context/AuthContext'

const SYMBOLS = ["AAPL", "MSFT", "NVDA", "TSLA"];
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

function AppContent() {
  const { user, logout, toggleAuthModal, setUser } = useAuth();
  const [activeSymbol, setActiveSymbol] = useState("AAPL");
  const [orderBook, setOrderBook] = useState({ bids: [], asks: [] });
  const [currentPrices, setCurrentPrices] = useState({ AAPL: 0, MSFT: 0, NVDA: 0, TSLA: 0 });
  const [histories, setHistories] = useState({ AAPL: [], MSFT: [], NVDA: [], TSLA: [] });
  const [price, setPrice] = useState("");
  const [quantity, setQuantity] = useState("");
  const [isBuy, setIsBuy] = useState(true);
  const [profitLoss, setProfitLoss] = useState({ profit: 0, loss: 0, net: 0, loading: true, error: null });

  const currentPrice = currentPrices[activeSymbol] || 0;
  const priceHistory = histories[activeSymbol] || [];

  // Fetch profit and loss data for the authenticated user
  useEffect(() => {
    const fetchProfitLoss = async () => {
      try {
        setProfitLoss(prev => ({ ...prev, loading: true, error: null }));

        // Use system_bot if no user is logged in, otherwise use user ID
        const userId = user ? user.id : "system_bot";

        const res = await fetch(`${API_URL}/profit-loss?user_id=${userId}`);
        if (res.ok) {
          const data = await res.json();
          setProfitLoss({
            profit: data.profit,
            loss: data.loss,
            net: data.net,
            loading: false,
            error: null
          });
        } else {
          setProfitLoss(prev => ({ ...prev, loading: false, error: "Failed to fetch profit/loss data" }));
        }
      } catch (err) {
        console.error("Failed to fetch profit/loss:", err);
        setProfitLoss(prev => ({ ...prev, loading: false, error: "Failed to fetch profit/loss data" }));
      }
    };

    fetchProfitLoss();
    const interval = setInterval(fetchProfitLoss, 5000); // Refresh every 5 seconds
    return () => clearInterval(interval);
  }, [user]);

  // Fetch data every second
  useEffect(() => {
    const fetchOrderBook = async () => {
      try {
        const res = await fetch(`${API_URL}/book?symbol=${activeSymbol}`);
        if (res.ok) {
          const data = await res.json();
          setOrderBook(data);

          const bestAsk = data.asks?.length > 0 ? data.asks[0].price : null;
          const bestBid = data.bids?.length > 0 ? data.bids[0].price : null;

          let newPrice = 0;
          if (bestAsk && bestBid) {
            newPrice = (bestAsk + bestBid) / 2;
          } else if (bestAsk) {
            newPrice = bestAsk;
          } else if (bestBid) {
            newPrice = bestBid;
          }

          if (newPrice > 0) {
            setCurrentPrices(prev => ({
              ...prev,
              [activeSymbol]: newPrice
            }));

            setHistories(prev => {
              const symbolHistory = prev[activeSymbol] || [];
              const timeString = new Date().toLocaleTimeString([], { hour12: false, minute: '2-digit', second: '2-digit' });
              return {
                ...prev,
                [activeSymbol]: [...symbolHistory, { time: timeString, price: newPrice }].slice(-30)
              };
            });
          }
        }
      } catch (err) {
        console.error("Failed to fetch order book:", err);
      }
    };

    fetchOrderBook();
    const interval = setInterval(fetchOrderBook, 1000);
    return () => clearInterval(interval);
  }, [activeSymbol]);

  const submitOrder = async (e) => {
    e.preventDefault();
    if (!price || !quantity) return;

    const order = {
      symbol: activeSymbol,
      isBuy: isBuy,
      quantity: parseInt(quantity),
      price: parseFloat(price)
    };

    // Add user_id if user is authenticated
    if (user && user.id) {
      order.user_id = user.id;
    }

    try {
      const res = await fetch(`${API_URL}/order`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(order)
      });

        if (res.ok) {
          setPrice("");
          setQuantity("");
          // Refresh user data to get updated balance
          const token = localStorage.getItem('token');
          if (token) {
            try {
              const response = await fetch(`${API_URL}/auth/me`, {
                headers: {
                  'Authorization': `Bearer ${token}`
                }
              });
          if (response.ok) {
            const data = await response.json();
            // Update user in context
            if (user && user.id) {
              // Create updated user object with new balance
              const updatedUser = {...user, balance: data.user.balance};
              // Update the user context to trigger a re-render
              setUser(updatedUser);
            }
          }
            } catch (error) {
              console.error('Failed to refresh user data:', error);
            }
          }
        } else {
          alert("Failed to place order.");
        }
    } catch (err) {
      console.error("API error:", err);
    }
  };

  return (
    <div style={{ minHeight: '100vh', backgroundColor: theme.bg, color: theme.textMain, padding: '20px', fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif' }}>
      <div style={{ maxWidth: '1100px', margin: '0 auto' }}>
        {/* HEADER: Main Title and Auth Controls */}
        <header style={{ marginBottom: '20px', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <h1 style={{ margin: 0, fontSize: '32px', fontWeight: '800', letterSpacing: '-0.5px' }}>Nexus Exchange</h1>

          <div style={{ display: 'flex', gap: '12px', alignItems: 'center' }}>
            {user ? (
              <>
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                  <div style={{
                    width: '32px', height: '32px', backgroundColor: theme.accent,
                    borderRadius: '50%', display: 'flex', alignItems: 'center',
                    justifyContent: 'center', color: 'white', fontWeight: 'bold'
                  }}>
                    {user.username.charAt(0).toUpperCase()}
                  </div>
                  <span style={{ color: theme.textMain, fontWeight: '500' }}>{user.username}</span>
                </div>
                <div style={{
                  backgroundColor: theme.panel, padding: '8px 16px', borderRadius: '6px',
                  border: `1px solid ${theme.border}`, fontSize: '14px'
                }}>
                  <span style={{ color: theme.textMuted, fontSize: '12px' }}>Balance: </span>
                  <span style={{ color: theme.textMain, fontWeight: '600' }}>${user.balance?.toFixed(2) || '0.00'}</span>
                </div>
                <button
                  onClick={logout}
                  style={{
                    padding: '8px 16px', backgroundColor: theme.sell, color: 'white',
                    border: 'none', borderRadius: '6px', cursor: 'pointer',
                    fontSize: '14px', fontWeight: '600'
                  }}
                >
                  Logout
                </button>
              </>
            ) : (
              <button
                onClick={toggleAuthModal}
                style={{
                  padding: '8px 16px', backgroundColor: theme.buy, color: 'white',
                  border: 'none', borderRadius: '6px', cursor: 'pointer',
                  fontSize: '14px', fontWeight: '600'
                }}
              >
                Login / Register
              </button>
            )}
          </div>
        </header>

        {/* TOP BAR: Selector and Live Price */}
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', backgroundColor: theme.panel, padding: '15px 25px', borderRadius: '8px', marginBottom: '20px', border: `1px solid ${theme.border}` }}>
          <div style={{ display: 'flex', gap: '10px' }}>
            {SYMBOLS.map(sym => (
              <button
                key={sym}
                onClick={() => setActiveSymbol(sym)}
                style={{
                  padding: '8px 16px',
                  backgroundColor: activeSymbol === sym ? theme.accent : 'transparent',
                  color: activeSymbol === sym ? '#fff' : theme.textMuted,
                  border: activeSymbol === sym ? `1px solid ${theme.accent}` : `1px solid ${theme.border}`,
                  borderRadius: '6px',
                  cursor: 'pointer',
                  fontWeight: '600',
                  transition: 'all 0.2s'
                }}
              >
                {sym}
              </button>
            ))}
          </div>

          <div style={{ textAlign: 'right' }}>
            <div style={{ fontSize: '12px', color: theme.textMuted, textTransform: 'uppercase', letterSpacing: '1px', marginBottom: '4px' }}>
              {activeSymbol} / USD
            </div>
            <div style={{ fontSize: '28px', fontWeight: 'bold', color: currentPrice > 0 ? theme.buy : theme.textMuted }}>
              {currentPrice > 0 ? `$${currentPrice.toFixed(2)}` : '---'}
            </div>
          </div>
        </div>

        {/* CHART */}
        <div style={{ width: '100%', height: 320, marginBottom: '20px', backgroundColor: theme.panel, borderRadius: '8px', padding: '20px', boxSizing: 'border-box', border: `1px solid ${theme.border}` }}>
          {priceHistory.length > 1 ? (
            <ResponsiveContainer width="100%" height="100%">
              <LineChart data={priceHistory}>
                <CartesianGrid strokeDasharray="3 3" vertical={false} stroke={theme.border} />
                <XAxis dataKey="time" tick={{fontSize: 12, fill: theme.textMuted}} stroke={theme.border} tickMargin={10} />
                <YAxis
                  domain={['dataMin - 1', 'dataMax + 1']}
                  tickFormatter={(value) => `$${value}`}
                  width={70}
                  tick={{fill: theme.textMuted, fontSize: 12}}
                  stroke={theme.border}
                  orientation="right"
                />
                <Tooltip
                  formatter={(value) => [`$${value}`, "Price"]}
                  contentStyle={{ backgroundColor: theme.panel, border: `1px solid ${theme.border}`, color: '#fff', borderRadius: '4px' }}
                />
                <Line
                  type="stepAfter"
                  dataKey="price"
                  stroke={theme.accent}
                  strokeWidth={2}
                  dot={false}
                  isAnimationActive={false}
                />
              </LineChart>
            </ResponsiveContainer>
          ) : (
            <div style={{ height: '100%', display: 'flex', alignItems: 'center', justifyContent: 'center', color: theme.textMuted }}>
              Gathering market data for {activeSymbol}...
            </div>
          )}
        </div>

        {/* PROFIT & LOSS DASHBOARD */}
        <div style={{ width: '100%', marginBottom: '20px', display: 'flex', gap: '20px', flexWrap: 'wrap' }}>
          {/* Profit Card */}
          <div style={{ flex: '1 1 300px', border: `1px solid ${theme.buy}`, padding: '20px', borderRadius: '8px', backgroundColor: theme.panel }}>
            <div style={{ fontSize: '12px', color: theme.textMuted, textTransform: 'uppercase', letterSpacing: '1px', marginBottom: '10px' }}>
              {user ? `${user.username}'s Profit` : 'Total Profit'}
            </div>
            <div style={{ fontSize: '28px', fontWeight: 'bold', color: theme.buy }}>
              {profitLoss.loading ? 'Loading...' : `$${profitLoss.profit.toFixed(2)}`}
            </div>
          </div>

          {/* Loss Card */}
          <div style={{ flex: '1 1 300px', border: `1px solid ${theme.sell}`, padding: '20px', borderRadius: '8px', backgroundColor: theme.panel }}>
            <div style={{ fontSize: '12px', color: theme.textMuted, textTransform: 'uppercase', letterSpacing: '1px', marginBottom: '10px' }}>
              {user ? `${user.username}'s Loss` : 'Total Loss'}
            </div>
            <div style={{ fontSize: '28px', fontWeight: 'bold', color: theme.sell }}>
              {profitLoss.loading ? 'Loading...' : `$${Math.abs(profitLoss.loss).toFixed(2)}`}
            </div>
          </div>

          {/* Net Result Card */}
          <div style={{ flex: '1 1 300px', border: `1px solid ${profitLoss.net >= 0 ? theme.buy : theme.sell}`, padding: '20px', borderRadius: '8px', backgroundColor: theme.panel }}>
            <div style={{ fontSize: '12px', color: theme.textMuted, textTransform: 'uppercase', letterSpacing: '1px', marginBottom: '10px' }}>
              {user ? `${user.username}'s Net Result` : 'Net Result'}
            </div>
            <div style={{ fontSize: '28px', fontWeight: 'bold', color: profitLoss.net >= 0 ? theme.buy : theme.sell }}>
              {profitLoss.loading ? 'Loading...' : (
                <>
                  {profitLoss.net >= 0 ? '+' : '-'}${Math.abs(profitLoss.net).toFixed(2)}
                </>
              )}
            </div>
          </div>
        </div>

        {/* BOTTOM PANELS: Order Book and Form */}
        <div style={{ display: 'flex', gap: '20px', flexWrap: 'wrap' }}>
          {/* ORDER BOOK */}
          <div style={{ flex: '1 1 400px', border: `1px solid ${theme.border}`, padding: '20px', borderRadius: '8px', backgroundColor: theme.panel }}>
            <h3 style={{ marginTop: 0, borderBottom: `1px solid ${theme.border}`, paddingBottom: '15px', fontSize: '16px' }}>Order Book</h3>

            {/* Header des colonnes */}
            <div style={{ display: 'flex', justifyContent: 'space-between', color: theme.textMuted, fontSize: '12px', marginBottom: '15px' }}>
              <span style={{ flex: 1, textAlign: 'left' }}>Side</span>
              <span style={{ flex: 1, textAlign: 'center' }}>Price (USD)</span>
              <span style={{ flex: 1, textAlign: 'right' }}>Amount</span>
            </div>

            {/* Asks (Sellers) */}
            <div style={{ marginBottom: '10px' }}>
              {orderBook.asks?.length > 0 ? orderBook.asks.slice(0, 8).reverse().map((ask, idx) => (
                <div key={`ask-${idx}`} style={{ display: 'flex', justifyContent: 'space-between', fontFamily: '"Roboto Mono", monospace', fontSize: '14px', margin: '6px 0' }}>
                  <span style={{ flex: 1, textAlign: 'left', color: theme.sell, fontWeight: 'bold' }}>Sell</span>
                  <span style={{ flex: 1, textAlign: 'center', color: theme.sell }}>{ask.price.toFixed(2)}</span>
                  <span style={{ flex: 1, textAlign: 'right', color: theme.textMain }}>{ask.quantity}</span>
                </div>
              )) : <div style={{ fontSize: '13px', color: theme.textMuted, textAlign: 'center', margin: '10px 0' }}>No asks</div>}
            </div>

            {/* Spread / Mid-Price divider */}
            <div style={{ textAlign: 'center', padding: '12px 0', borderTop: `1px dashed ${theme.border}`, borderBottom: `1px dashed ${theme.border}`, margin: '15px 0', color: currentPrice > 0 ? theme.textMain : theme.textMuted, fontSize: '18px', fontWeight: 'bold' }}>
              {currentPrice > 0 ? `$${currentPrice.toFixed(2)}` : 'Spread'}
            </div>

            {/* Bids (Buyers) */}
            <div>
              {orderBook.bids?.length > 0 ? orderBook.bids.slice(0, 8).map((bid, idx) => (
                <div key={`bid-${idx}`} style={{ display: 'flex', justifyContent: 'space-between', fontFamily: '"Roboto Mono", monospace', fontSize: '14px', margin: '6px 0' }}>
                  <span style={{ flex: 1, textAlign: 'left', color: theme.buy, fontWeight: 'bold' }}>Buy</span>
                  <span style={{ flex: 1, textAlign: 'center', color: theme.buy }}>{bid.price.toFixed(2)}</span>
                  <span style={{ flex: 1, textAlign: 'right', color: theme.textMain }}>{bid.quantity}</span>
                </div>
              )) : <div style={{ fontSize: '13px', color: theme.textMuted, textAlign: 'center', margin: '10px 0' }}>No bids</div>}
            </div>
          </div>

          {/* TRADING FORM */}
          <div style={{ flex: '1 1 400px', border: `1px solid ${theme.border}`, padding: '20px', borderRadius: '8px', backgroundColor: theme.panel }}>
            <h3 style={{ marginTop: 0, borderBottom: `1px solid ${theme.border}`, paddingBottom: '15px', fontSize: '16px' }}>Place Order</h3>

            <form onSubmit={submitOrder} style={{ display: 'flex', flexDirection: 'column', gap: '20px', marginTop: '20px' }}>
              {/* Toggle Buy / Sell Buttons */}
              <div style={{ display: 'flex', gap: '10px' }}>
                <button
                  type="button"
                  onClick={() => setIsBuy(true)}
                  style={{ flex: 1, padding: '10px', borderRadius: '4px', border: 'none', cursor: 'pointer', fontWeight: 'bold', backgroundColor: isBuy ? theme.buy : theme.bg, color: isBuy ? '#fff' : theme.textMuted, border: `1px solid ${isBuy ? theme.buy : theme.border}` }}
                >
                  Buy
                </button>
                <button
                  type="button"
                  onClick={() => setIsBuy(false)}
                  style={{ flex: 1, padding: '10px', borderRadius: '4px', border: 'none', cursor: 'pointer', fontWeight: 'bold', backgroundColor: !isBuy ? theme.sell : theme.bg, color: !isBuy ? '#fff' : theme.textMuted, border: `1px solid ${!isBuy ? theme.sell : theme.border}` }}
                >
                  Sell
                </button>
              </div>

              {/* Input Price */}
              <div style={{ display: 'flex', flexDirection: 'column' }}>
                <label style={{ fontSize: '12px', color: theme.textMuted, marginBottom: '5px' }}>Price (USD)</label>
                <div style={{ display: 'flex', alignItems: 'center', backgroundColor: theme.bg, border: `1px solid ${theme.border}`, borderRadius: '4px', padding: '0 10px' }}>
                  <span style={{ color: theme.textMuted }}>$</span>
                  <input
                    type="number"
                    step="0.01"
                    value={price}
                    onChange={(e) => setPrice(e.target.value)}
                    required
                    style={{ flex: 1, padding: '12px 10px', backgroundColor: 'transparent', border: 'none', color: '#fff', outline: 'none', fontSize: '14px' }}
                  />
                </div>
              </div>

              {/* Input Quantity */}
              <div style={{ display: 'flex', flexDirection: 'column' }}>
                <label style={{ fontSize: '12px', color: theme.textMuted, marginBottom: '5px' }}>Amount</label>
                <input
                  type="number"
                  value={quantity}
                  onChange={(e) => setQuantity(e.target.value)}
                  required
                  style={{ padding: '12px', borderRadius: '4px', border: `1px solid ${theme.border}`, backgroundColor: theme.bg, color: '#fff', outline: 'none', fontSize: '14px' }}
                />
              </div>

              {/* Submit Button */}
              <button type="submit" style={{
                padding: '14px',
                backgroundColor: isBuy ? theme.buy : theme.sell,
                color: 'white',
                border: 'none',
                borderRadius: '4px',
                cursor: 'pointer',
                fontWeight: 'bold',
                fontSize: '16px',
                marginTop: '10px',
                transition: 'opacity 0.2s'
              }}
              onMouseOver={(e) => e.target.style.opacity = 0.8}
              onMouseOut={(e) => e.target.style.opacity = 1}
              >
                {isBuy ? 'Buy ' : 'Sell '} {activeSymbol}
              </button>
            </form>
          </div>
        </div>
      </div>
    </div>
  );
}

export default AppContent;