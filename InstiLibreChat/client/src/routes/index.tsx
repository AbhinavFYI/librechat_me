import { createBrowserRouter, Navigate, Outlet } from 'react-router-dom';
import {
  Login,
  OTP,
  VerifyEmail,
  Registration,
  ResetPassword,
  ApiErrorWatcher,
  TwoFactorScreen,
  RequestPasswordReset,
} from '~/components/Auth';
import { MarketplaceProvider } from '~/components/Agents/MarketplaceContext';
import AgentMarketplace from '~/components/Agents/Marketplace';
import { OAuthSuccess, OAuthError } from '~/components/OAuth';
import { AuthContextProvider } from '~/hooks/AuthContext';
import RouteErrorBoundary from './RouteErrorBoundary';
import StartupLayout from './Layouts/Startup';
import LoginLayout from './Layouts/Login';
import dashboardRoutes from './Dashboard';
import ShareRoute from './ShareRoute';
import ChatRoute from './ChatRoute';
import Search from './Search';
import Root from './Root';
import AdminRoute from './AdminRoute';
import TemplatesRoute from './TemplatesRoute';
import ScreenerRoute from './ScreenerRoute';
import ResourcesRoute from './ResourcesRoute';
import FileViewRoute from './FileViewRoute';

// Component to handle static file routes (returns null to prevent React Router from handling them)
const StaticFileRoute = () => {
  // This route is handled by the backend, not React Router
  // Return null to prevent React Router from trying to render anything
  return null;
};

const AuthLayout = () => (
  <Outlet />
);

// Wrap all routes with AuthContextProvider and ApiErrorWatcher
const RootLayout = () => (
  <AuthContextProvider>
    <Outlet />
    <ApiErrorWatcher />
  </AuthContextProvider>
);

const baseEl = document.querySelector('base');
const baseHref = baseEl?.getAttribute('href') || '/';

export const router = createBrowserRouter(
  [
    {
      element: <RootLayout />,
      errorElement: <RouteErrorBoundary />,
      children: [
    // Static file route - must be first to prevent React Router from trying to handle backend routes
    {
      path: 'static/*',
      element: <StaticFileRoute />,
    },
    {
      path: 'share/:shareId',
      element: <ShareRoute />,
      errorElement: <RouteErrorBoundary />,
    },
    {
      path: 'oauth',
      errorElement: <RouteErrorBoundary />,
      children: [
        {
          path: 'success',
          element: <OAuthSuccess />,
        },
        {
          path: 'error',
          element: <OAuthError />,
        },
      ],
    },
    {
      path: '/',
      element: <StartupLayout />,
      errorElement: <RouteErrorBoundary />,
      children: [
        {
          path: 'register',
          element: <Registration />,
        },
        {
          path: 'forgot-password',
          element: <RequestPasswordReset />,
        },
        {
          path: 'reset-password',
          element: <ResetPassword />,
        },
      ],
    },
    {
      path: 'verify',
      element: <VerifyEmail />,
      errorElement: <RouteErrorBoundary />,
    },
    {
      element: <AuthLayout />,
      errorElement: <RouteErrorBoundary />,
      children: [
        {
          path: '/',
          element: <LoginLayout />,
          children: [
            {
              path: 'login',
              element: <Login />,
            },
            {
              path: 'login/otp',
              element: <OTP />,
            },
            {
              path: 'login/2fa',
              element: <TwoFactorScreen />,
            },
          ],
        },
        dashboardRoutes,
        {
          path: '/',
          element: <Root />,
          children: [
            {
              index: true,
              element: <Navigate to="/c/new" replace={true} />,
            },
            {
              path: 'c/:conversationId?',
              element: <ChatRoute />,
            },
            {
              path: 'search',
              element: <Search />,
            },
            {
              path: 'agents',
              element: (
                <MarketplaceProvider>
                  <AgentMarketplace />
                </MarketplaceProvider>
              ),
            },
            {
              path: 'agents/:category',
              element: (
                <MarketplaceProvider>
                  <AgentMarketplace />
                </MarketplaceProvider>
              ),
            },
            {
              path: 'admin',
              element: <AdminRoute />,
            },
            {
              path: 'resources',
              element: <ResourcesRoute />,
            },
            {
              path: 'templates',
              element: <TemplatesRoute />,
            },
            {
              path: 'screener',
              element: <ScreenerRoute />,
            },
          ],
        },
      ],
    },
        // File view route - now wrapped by AuthContextProvider but outside AuthLayout for direct file viewing
    {
      path: 'files/:fileId',
      element: <FileViewRoute />,
      errorElement: <RouteErrorBoundary />,
        },
      ],
    },
  ],
  { basename: baseHref },
);
