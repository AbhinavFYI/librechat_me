import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { TrendingUp, BarChart3, Shield, Users, FileText, MessageSquare, Check } from 'lucide-react';
import { useToastContext } from '@librechat/client';
import { useOutletContext } from 'react-router-dom';
import type { TLoginLayoutContext } from '~/common';

function Login() {
  const navigate = useNavigate();
  const { showToast } = useToastContext();
  const { startupConfig } = useOutletContext<TLoginLayoutContext>();
  const [email, setEmail] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const companyName = startupConfig?.appTitle || 'FYERS Securities Pvt Ltd.';

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!email || !email.includes('@')) {
      showToast({
        message: 'Please enter a valid email address',
        status: 'error',
      });
      return;
    }

    setIsLoading(true);
    try {
      // Use relative URL so it works on any domain (localhost or production)
      const response = await fetch('/api/v1/auth/send-otp', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ email }),
        credentials: 'include', // Include cookies for auth
      });

      const data = await response.json();

      if (!response.ok) {
        if (response.status === 403 && data.error === 'UNAUTHORIZED') {
          showToast({
            message: 'Access Denied: Only super admins and organization admins can login via OTP. Please contact your administrator.',
            status: 'error',
          });
        } else if (response.status === 404 && data.error === 'USER_NOT_FOUND') {
          showToast({
            message: 'User not found. Please check your email address.',
            status: 'error',
          });
        } else {
          showToast({
            message: data.message || 'Failed to send OTP',
            status: 'error',
          });
    }
        setIsLoading(false);
        return;
      }

      // OTP sent successfully, navigate to OTP page
      navigate(`/login/otp?email=${encodeURIComponent(email)}`);
    } catch (error) {
      console.error('Error sending OTP:', error);
      showToast({
        message: 'Failed to send OTP. Please try again.',
        status: 'error',
      });
      setIsLoading(false);
    }
  };

    return (
    <div className="flex min-h-screen">
      {/* Left Section - Blue Gradient */}
      <div className="flex-1 bg-gradient-to-br from-[#4158D0] to-[#5B6FD8] p-12 flex flex-col">
        {/* Logo */}
        <div className="flex items-center gap-3 mb-20">
          <div className="w-10 h-10 bg-white rounded-lg flex items-center justify-center">
            <span className="text-[#4158D0] text-2xl font-bold">F</span>
          </div>
          <span className="text-white text-xl font-medium">FIA - FYERS Intelligent Assistant</span>
        </div>

        <div style={{ maxWidth: '1400px' }}>
          {/* Main Headline */}
          <h1 className="text-white text-5xl font-bold leading-tight mb-8">
            Institutional-grade research<br />with the power of AI.
          </h1>

          {/* Feature List */}
          <div className="space-y-4 mb-12">
            <div className="flex items-center gap-3 text-white text-lg">
              <BarChart3 className="w-5 h-5" />
              <span>AI-powered institutional research</span>
            </div>
            <div className="flex items-center gap-3 text-white text-lg">
              <Shield className="w-5 h-5" />
              <span>Secure document intelligence</span>
            </div>
            <div className="flex items-center gap-3 text-white text-lg">
              <Users className="w-5 h-5" />
              <span>Collaborative reporting for teams</span>
            </div>
          </div>

          {/* Interactive Cards */}
          <div className="bg-white/10 backdrop-blur-sm rounded-2xl p-6" style={{ width: '1000px', maxWidth: '100%' }}>
            <div className="flex gap-8 mb-6" style={{ justifyContent: 'center' }}>
              {/* Document Analysis Card */}
              <div className="bg-white rounded-xl p-6 flex flex-col" style={{ width: '450px', aspectRatio: '1.8' }}>
                <div className="flex items-center justify-between mb-4">
                  <div className="flex items-center gap-2">
                    <div className="w-8 h-8 bg-blue-500 rounded-lg flex items-center justify-center">
                      <FileText className="w-5 h-5 text-white" />
                    </div>
                    <span className="font-semibold text-gray-900">Document Analysis</span>
                  </div>
                  <span className="text-xs font-medium text-gray-500 bg-gray-100 px-2 py-1 rounded">NEW</span>
                </div>
                <div className="space-y-2 mb-4 flex-1">
                  <div className="h-2 bg-gray-200 rounded w-full"></div>
                  <div className="h-2 bg-gray-200 rounded w-full"></div>
                  <div className="h-2 bg-gray-200 rounded w-3/4"></div>
                  <div className="h-2 bg-gray-200 rounded w-1/2"></div>
                </div>
                <div className="bg-green-50 border border-green-200 rounded-lg p-3 flex items-center gap-2">
                  <Check className="w-4 h-4 text-green-600" />
                  <span className="text-sm text-green-800">Document analyzed successfully</span>
                </div>
              </div>

              {/* Stocks on Chat Card */}
              <div className="bg-white rounded-xl p-6 flex flex-col" style={{ width: '450px', aspectRatio: '1.8' }}>
                <div className="flex items-center gap-2 mb-4">
                  <div className="w-8 h-8 bg-purple-500 rounded-lg flex items-center justify-center">
                    <MessageSquare className="w-5 h-5 text-white" />
                  </div>
                  <span className="font-semibold text-gray-900">Stocks on Chat</span>
                </div>
                <div className="space-y-3 flex-1">
                  <div className="bg-gray-50 rounded-lg p-3">
                    <p className="text-sm text-gray-600">What's the main topic?</p>
                  </div>
                  <div className="bg-blue-500 rounded-lg p-3">
                    <p className="text-sm text-white">The document discusses AI implementation strategies...</p>
                  </div>
                  <div className="bg-gray-50 rounded-lg p-3">
                    <p className="text-sm text-gray-600">Can you summarize key points?</p>
                  </div>
                </div>
              </div>
            </div>

            {/* Bottom Navigation */}
            <div className="flex items-center justify-center gap-6 text-white">
              <button className="flex items-center gap-2 hover:opacity-80 transition">
                <FileText className="w-4 h-4" />
                <span className="text-sm">DocuAI Analyser</span>
              </button>
              <button className="flex items-center gap-2 hover:opacity-80 transition">
                <MessageSquare className="w-4 h-4" />
                <span className="text-sm">DocuAI Chat</span>
              </button>
            </div>
          </div>
        </div>
      </div>

      {/* Right Section - Login Form */}
      <div className="w-[480px] bg-white p-12 flex flex-col justify-center">
        <div className="max-w-md">
          <h2 className="text-3xl font-bold text-gray-900 mb-2">Login to FIA</h2>
          <p className="text-gray-600 mb-8">{companyName}</p>

          <form onSubmit={handleSubmit} className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">Email</label>
              <input
                type="email"
                placeholder="Enter your email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                className="w-full px-4 py-3 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
                required
                disabled={isLoading}
              />
            </div>

            <button 
              type="submit"
              disabled={isLoading || !email}
              className={`w-full py-3 rounded-lg font-medium transition ${
                isLoading || !email
                  ? 'bg-gray-200 text-gray-400 cursor-not-allowed'
                  : 'bg-[#60A5FA] text-white hover:bg-[#3B82F6]'
              }`}
          >
              {isLoading ? 'Sending...' : 'Send OTP'}
            </button>
          </form>
        </div>
      </div>
    </div>
  );
}

export default Login;
