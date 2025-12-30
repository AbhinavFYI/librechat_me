import { useState, useEffect } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { TrendingUp, BarChart3, Shield, Users, FileText, MessageSquare, Check, ArrowLeft } from 'lucide-react';
import { useToastContext } from '@librechat/client';
import { useOutletContext } from 'react-router-dom';
import type { TLoginLayoutContext } from '~/common';

function OTP() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const { showToast } = useToastContext();
  const { startupConfig } = useOutletContext<TLoginLayoutContext>();
  const email = searchParams.get('email') || '';
  const companyName = startupConfig?.appTitle || 'FYERS Securities Pvt Ltd.';
  
  const [otp, setOtp] = useState(['', '', '', '', '', '']);
  const [isLoading, setIsLoading] = useState(false);
  const [isResending, setIsResending] = useState(false);

  const handleOtpChange = (index: number, value: string) => {
    if (value.length <= 1 && /^\d*$/.test(value)) {
      const newOtp = [...otp];
      newOtp[index] = value;
      setOtp(newOtp);
      
      // Auto-focus next input
      if (value && index < 5) {
        setTimeout(() => {
          const nextInput = document.getElementById(`otp-${index + 1}`);
          if (nextInput) nextInput.focus();
        }, 10);
      }
    }
  };

  const handleKeyDown = (index: number, e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Backspace' && !otp[index] && index > 0) {
      const prevInput = document.getElementById(`otp-${index - 1}`);
      if (prevInput) prevInput.focus();
    }
    
    // Handle Enter key to submit OTP
    if (e.key === 'Enter') {
      const otpValue = otp.join('');
      if (otpValue.length === 6 && !isLoading) {
        e.preventDefault();
        handleLogin();
      }
    }
  };

  const handleLogin = async () => {
    const otpValue = otp.join('');
    if (otpValue.length !== 6) {
      showToast({
        message: 'Please enter complete OTP',
        status: 'error',
      });
      return;
    }

    setIsLoading(true);
    try {
      // Use relative URL so it works on any domain (localhost or production)
      const response = await fetch('/api/v1/auth/verify-otp', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ email, otp: otpValue }),
        credentials: 'include', // Include cookies for auth
      });

      const responseData = await response.json();

      if (!response.ok) {
        showToast({
          message: responseData.message || 'Invalid OTP',
          status: 'error',
        });
        setIsLoading(false);
        return;
      }

      // Handle both response formats:
      // 1. insti-inquora: { code: 200, s: "ok", data: { access_token, refresh_token, user, permissions } }
      // 2. saas-api: { access_token, refresh_token, user, permissions }
      const data = responseData.data || responseData;

      // Store tokens
      if (data.access_token) {
        localStorage.setItem('access_token', data.access_token);
        localStorage.setItem('refresh_token', data.refresh_token);
        const userData = data.user || data;
        localStorage.setItem('user', JSON.stringify(userData));
        
        // Store permissions
        if (data.permissions) {
          localStorage.setItem('permissions', JSON.stringify(data.permissions));
        }
        
        // Check and store admin access status immediately during login
        // Based on org_role: show admin panel if org_role === 'admin' OR is_super_admin === true
        const isSuperAdmin = userData.is_super_admin === true;
        const orgRole = userData.org_role || userData.orgRole;
        const isOrgAdmin = orgRole === 'admin';
        
        const canAccessAdmin = isSuperAdmin || isOrgAdmin;
        localStorage.setItem('canAccessAdmin', JSON.stringify(canAccessAdmin));
        
        // Step 1: Login to proxy server - this syncs user to MongoDB and sets JWT cookie
        try {
          // Ensure we have a valid email
          const userEmail = userData?.email || email;
          if (!userEmail) {
            throw new Error('Email is required for proxy login');
          }

          const proxyLoginPayload: any = { email: userEmail };
          // Only include refresh_token if it exists and is not empty
          if (data.refresh_token && data.refresh_token.trim() !== '') {
            proxyLoginPayload.refresh_token = data.refresh_token;
          }

          // Use relative URL so it goes to the same domain user is accessing
          // This ensures cookies are set for the correct domain (research.fyers.in, not localhost)
          const proxyLoginResponse = await fetch('/login', {
            method: 'POST',
            headers: {
              'Content-Type': 'application/json',
            },
            body: JSON.stringify(proxyLoginPayload),
            credentials: 'include',
          });
          
          if (!proxyLoginResponse.ok) {
            const errorText = await proxyLoginResponse.text();
            let errorMessage = 'Failed to sync with FIA. Please try again.';
            try {
              const errorData = JSON.parse(errorText);
              errorMessage = errorData.message || errorData.error || errorMessage;
            } catch {
              // If not JSON, use the text or status
              if (errorText) {
                errorMessage = errorText;
              } else if (proxyLoginResponse.status === 400) {
                errorMessage = 'Invalid request. Please check your email and try again.';
              }
            }
            console.error('Proxy login failed:', proxyLoginResponse.status, errorMessage);
            setIsLoading(false);
            showToast({
              message: errorMessage,
              status: 'error',
            });
            return;
          }
          
          console.log('Proxy login successful - user synced to MongoDB and cookies set');
          
          // Step 2: Wait for MongoDB sync and cookie setting to complete
          // The proxy has already:
          // 1. Synced user to MongoDB
          // 2. Created LibreChat session
          // 3. Set refreshToken, token_provider, and libre_jwt cookies
          // We don't need to call /api/auth/refresh because the proxy handles everything
          await new Promise(resolve => setTimeout(resolve, 2000));
          
          console.log('Redirecting to chat interface - authentication cookies are set');
          
          // Step 3: Redirect directly to chat interface
          // The AuthContext will automatically pick up the cookies and authenticate
          window.location.href = '/c/new';
          return;
        } catch (proxyError) {
          console.error('Proxy server error:', proxyError);
          setIsLoading(false);
          showToast({
            message: 'Failed to connect to FIA. Please try again.',
            status: 'error',
          });
          return;
        }
      }
      
      setIsLoading(false);
    } catch (error) {
      console.error('Error verifying OTP:', error);
      showToast({
        message: 'Failed to verify OTP. Please try again.',
        status: 'error',
      });
      setIsLoading(false);
    }
  };

  const handleResend = async () => {
    setIsResending(true);
    setOtp(['', '', '', '', '', '']);
    
    try {
      // Use relative URL so it works on any domain (localhost or production)
      const response = await fetch('/api/v1/auth/resend-otp', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ email }),
        credentials: 'include', // Include cookies for auth
      });

      const data = await response.json();

      if (!response.ok) {
        showToast({
          message: data.message || 'Failed to resend OTP',
          status: 'error',
        });
        setIsResending(false);
        return;
      }

      showToast({
        message: 'OTP resent to your email',
        status: 'success',
      });
    } catch (error) {
      console.error('Error resending OTP:', error);
      showToast({
        message: 'Failed to resend OTP. Please try again.',
        status: 'error',
      });
    } finally {
      setIsResending(false);
    }
  };

  if (!email) {
    // Redirect to login if no email
    navigate('/login');
    return null;
  }

  return (
    <div className="flex flex-col lg:flex-row min-h-screen overflow-x-hidden">
      {/* Left Section - Same Blue Gradient as Login */}
      <div className="flex-1 bg-gradient-to-br from-[#2434E7] to-[#3B4FE8] p-6 sm:p-8 lg:p-12 flex flex-col min-h-[40vh] lg:min-h-screen">
        {/* Logo */}
        <div className="flex items-center gap-2 sm:gap-3 mb-8 sm:mb-12 lg:mb-20">
          <div className="w-10 h-10 sm:w-12 sm:h-12 bg-white rounded-lg flex items-center justify-center p-1 flex-shrink-0">
            <img
              src="assets/Logo.svg"
              alt="FIA Logo"
              className="w-full h-full object-contain"
            />
          </div>
          <span className="text-white text-base sm:text-lg lg:text-xl font-medium">FIA - FYERS Intelligent Assistant</span>
        </div>

        <div className="max-w-full lg:max-w-[1400px]">
          {/* Main Headline */}
          <h1 className="text-white text-2xl sm:text-3xl lg:text-5xl font-bold leading-tight mb-6 lg:mb-8">
            Institutional-grade research<br />with the power of AI.
          </h1>

          {/* Feature List */}
          <div className="space-y-3 sm:space-y-4 mb-8 lg:mb-12">
            <div className="flex items-center gap-2 sm:gap-3 text-white text-sm sm:text-base lg:text-lg">
              <TrendingUp className="w-4 h-4 sm:w-5 sm:h-5 flex-shrink-0" />
              <span>AI-powered institutional research</span>
            </div>
            <div className="flex items-center gap-2 sm:gap-3 text-white text-sm sm:text-base lg:text-lg">
              <Shield className="w-4 h-4 sm:w-5 sm:h-5 flex-shrink-0" />
              <span>Secure document intelligence</span>
            </div>
            <div className="flex items-center gap-2 sm:gap-3 text-white text-sm sm:text-base lg:text-lg">
              <Users className="w-4 h-4 sm:w-5 sm:h-5 flex-shrink-0" />
              <span>Collaborative reporting for teams</span>
            </div>
          </div>

          {/* Interactive Cards - Hidden on mobile, visible on larger screens */}
          <div className="hidden md:block bg-white/10 backdrop-blur-sm rounded-2xl p-4 lg:p-6 max-w-full">
            <div className="flex flex-col lg:flex-row gap-4 lg:gap-8 mb-4 lg:mb-6 justify-center">
              {/* Document Analysis Card */}
              <div className="bg-white rounded-xl p-4 lg:p-6 flex flex-col w-full lg:w-[450px] lg:max-w-[450px]">
                <div className="flex items-center justify-between mb-3 lg:mb-4">
                  <div className="flex items-center gap-2">
                    <div className="w-7 h-7 lg:w-8 lg:h-8 bg-blue-500 rounded-lg flex items-center justify-center flex-shrink-0">
                      <FileText className="w-4 h-4 lg:w-5 lg:h-5 text-white" />
                    </div>
                    <span className="font-semibold text-gray-900 text-sm lg:text-base">Document Analysis</span>
                  </div>
                  <span className="text-xs font-medium text-gray-500 bg-gray-100 px-2 py-1 rounded">NEW</span>
                </div>
                <div className="space-y-2 mb-3 lg:mb-4 flex-1">
                  <div className="h-2 bg-gray-200 rounded w-full"></div>
                  <div className="h-2 bg-gray-200 rounded w-full"></div>
                  <div className="h-2 bg-gray-200 rounded w-3/4"></div>
                  <div className="h-2 bg-gray-200 rounded w-1/2"></div>
                </div>
                <div className="bg-green-50 border border-green-200 rounded-lg p-2 lg:p-3 flex items-center gap-2">
                  <Check className="w-4 h-4 text-green-600 flex-shrink-0" />
                  <span className="text-xs lg:text-sm text-green-800">Document analyzed successfully</span>
                </div>
              </div>

              {/* Stocks on Chat Card */}
              <div className="bg-white rounded-xl p-4 lg:p-6 flex flex-col w-full lg:w-[450px] lg:max-w-[450px]">
                <div className="flex items-center gap-2 mb-3 lg:mb-4">
                  <div className="w-7 h-7 lg:w-8 lg:h-8 bg-purple-500 rounded-lg flex items-center justify-center flex-shrink-0">
                    <MessageSquare className="w-4 h-4 lg:w-5 lg:h-5 text-white" />
                  </div>
                  <span className="font-semibold text-gray-900 text-sm lg:text-base">Stocks on Chat</span>
                </div>
                <div className="space-y-2 lg:space-y-3 flex-1">
                  <div className="bg-gray-50 rounded-lg p-2 lg:p-3">
                    <p className="text-xs lg:text-sm text-gray-600">What's the main topic?</p>
                  </div>
                  <div className="bg-blue-500 rounded-lg p-2 lg:p-3">
                    <p className="text-xs lg:text-sm text-white">The document discusses AI implementation strategies...</p>
                  </div>
                  <div className="bg-gray-50 rounded-lg p-2 lg:p-3">
                    <p className="text-xs lg:text-sm text-gray-600">Can you summarize key points?</p>
                  </div>
                </div>
              </div>
            </div>

            {/* Bottom Navigation */}
            <div className="flex items-center justify-center gap-4 lg:gap-6 text-white text-sm">
              <button className="flex items-center gap-2 hover:opacity-80 transition">
                <FileText className="w-4 h-4 flex-shrink-0" />
                <span>DocuAI Analyser</span>
              </button>
              <button className="flex items-center gap-2 hover:opacity-80 transition">
                <MessageSquare className="w-4 h-4 flex-shrink-0" />
                <span>DocuAI Chat</span>
              </button>
            </div>
          </div>
        </div>
      </div>

      {/* Right Section - OTP Form */}
      <div className="w-full lg:w-[500px] bg-white p-6 sm:p-8 lg:p-12 flex flex-col">
        {/* Back Button */}
        <button 
          className="flex items-center gap-2 text-gray-700 mb-6 sm:mb-8 hover:text-gray-900 transition self-start"
          onClick={() => navigate('/login')}
        >
          <ArrowLeft className="w-4 h-4 sm:w-5 sm:h-5 flex-shrink-0" />
          <span className="font-medium text-sm sm:text-base">Back</span>
        </button>

        <div className="flex-1 flex flex-col justify-center max-w-md mx-auto w-full">
          <h2 className="text-2xl sm:text-3xl font-bold text-gray-900 mb-2">Login to FIA</h2>
          <p className="text-gray-600 mb-2 text-sm sm:text-base">{companyName}</p>
          <p className="text-xs sm:text-sm text-gray-500 mb-6 sm:mb-8 break-all">OTP sent to: {email}</p>

          <div className="mb-6">
            <label className="block text-sm font-medium text-gray-700 mb-3">Enter OTP</label>
            <div className="flex gap-2 sm:gap-[14px] justify-center sm:justify-start">
              {otp.map((digit, index) => (
                <input
                  key={index}
                  id={`otp-${index}`}
                  type="text"
                  inputMode="numeric"
                  maxLength={1}
                  value={digit}
                  onChange={(e) => handleOtpChange(index, e.target.value)}
                  onKeyDown={(e) => handleKeyDown(index, e)}
                  className="w-10 h-10 sm:w-12 sm:h-12 lg:w-14 lg:h-14 border-2 border-gray-300 rounded-lg text-center text-base sm:text-lg font-semibold focus:border-[#2434E7] focus:outline-none"
                />
              ))}
            </div>
          </div>

          <button 
            className="w-full bg-[#2434E7] text-white font-semibold py-3 rounded-lg hover:bg-[#1a28b8] transition mb-4 disabled:bg-gray-400 disabled:cursor-not-allowed text-base"
            onClick={handleLogin}
            disabled={isLoading || otp.join('').length !== 6}
          >
            {isLoading ? 'Verifying...' : 'Login'}
          </button>

          <button 
            className="w-full text-gray-700 font-medium py-3 hover:text-gray-900 transition border border-gray-300 rounded-lg disabled:opacity-50 disabled:cursor-not-allowed text-base"
            onClick={handleResend}
            disabled={isResending}
          >
            {isResending ? 'Resending...' : 'Resend OTP'}
          </button>
        </div>
      </div>
    </div>
  );
}

export default OTP;

