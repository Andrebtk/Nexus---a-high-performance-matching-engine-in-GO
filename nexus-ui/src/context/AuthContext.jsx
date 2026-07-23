import { createContext, useContext, useState, useEffect } from 'react';
import { Auth } from '../components/Auth';

const API_URL = "http://localhost:8080";

const AuthContext = createContext();

export function AuthProvider({ children }) {
  const [user, setUser] = useState(null);
  const [loading, setLoading] = useState(true);
  const [showAuthModal, setShowAuthModal] = useState(false);

  // Check for existing session on app load
  useEffect(() => {
    const checkAuth = async () => {
      const token = localStorage.getItem('token');
      if (!token) {
        // No existing session, create a guest account
        await createGuestSession();
        return;
      }

      try {
        const response = await fetch(`${API_URL}/auth/me`, {
          headers: {
            'Authorization': `Bearer ${token}`
          }
        });

        if (response.ok) {
          const data = await response.json();
          setUser(data.user);
        } else {
          // Token is invalid, clear it and create guest
          localStorage.removeItem('token');
          localStorage.removeItem('user');
          await createGuestSession();
        }
      } catch (error) {
        console.error('Failed to check auth:', error);
        await createGuestSession();
      } finally {
        setLoading(false);
      }
    };

    checkAuth();
  }, []);

  const createGuestSession = async () => {
    try {
      const response = await fetch(`${API_URL}/auth/guest`);
      if (response.ok) {
        const data = await response.json();
        // Store guest token temporarily (not persisted)
        setUser(data.user);
      }
    } catch (error) {
      console.error('Failed to create guest session:', error);
      // Fallback to system_bot if guest creation fails
      setUser({
        id: "system_bot",
        username: "Guest",
        email: "guest@nexus.com",
        balance: 1000,
        profit: 0,
        loss: 0,
        created_at: new Date().toISOString()
      });
    }
  };

  const login = (userData) => {
    setUser(userData);
    setShowAuthModal(false);
  };

  const logout = () => {
    localStorage.removeItem('token');
    localStorage.removeItem('user');
    setUser(null);
  };

  const toggleAuthModal = () => {
    setShowAuthModal(!showAuthModal);
  };

  return (
    <AuthContext.Provider value={{
      user,
      loading,
      showAuthModal,
      setUser,
      login,
      logout,
      toggleAuthModal
    }}>
      {children}
      {showAuthModal && (
        <Auth onLoginSuccess={login} />
      )}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  return useContext(AuthContext);
}