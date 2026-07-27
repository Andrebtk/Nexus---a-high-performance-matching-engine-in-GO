import { createContext, useContext, useState, useEffect } from 'react';
import { Auth } from '../components/Auth';

const API_URL = "http://localhost:8080";

const AuthContext = createContext();

export function AuthProvider({ children }) {
  const [user, setUser] = useState(null);
  const [loading, setLoading] = useState(true);
  const [showAuthModal, setShowAuthModal] = useState(false);
  const [isAuthenticated, setIsAuthenticated] = useState(false);

  // Check for existing session on app load
  useEffect(() => {
    const checkAuth = async () => {
      const token = localStorage.getItem('token');
      if (!token) {
        // No existing session, set user to null (no guest creation)
        setUser(null);
        setIsAuthenticated(false);
        setLoading(false);
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
          setIsAuthenticated(true);
        } else {
          // Token is invalid, clear it
          localStorage.removeItem('token');
          localStorage.removeItem('user');
          setUser(null);
          setIsAuthenticated(false);
        }
      } catch (error) {
        console.error('Failed to check auth:', error);
        setUser(null);
        setIsAuthenticated(false);
      } finally {
        setLoading(false);
      }
    };

    checkAuth();
  }, []);

  const login = (userData) => {
    setUser(userData);
    setIsAuthenticated(true);
    setShowAuthModal(false);
  };

  const logout = () => {
    localStorage.removeItem('token');
    localStorage.removeItem('user');
    setUser(null);
    setIsAuthenticated(false);
  };

  const toggleAuthModal = () => {
    setShowAuthModal(!showAuthModal);
  };

      return (
        <AuthContext.Provider value={{
          user,
          loading,
          showAuthModal,
          isAuthenticated,
          setUser,
          login,
          logout,
          toggleAuthModal
        }}>
          {children}
          {showAuthModal && (
            <Auth onLoginSuccess={login} onClose={toggleAuthModal} />
          )}
        </AuthContext.Provider>
      );
}

export function useAuth() {
  return useContext(AuthContext);
}