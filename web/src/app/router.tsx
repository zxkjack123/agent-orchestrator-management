import { createBrowserRouter } from 'react-router-dom'
import { Layout } from '@/components/Layout'
import { WarRoom } from '@/features/war-room/WarRoom'
import { SessionsView } from '@/features/sessions/SessionsView'
import { TasksView } from '@/features/tasks/TasksView'
import { MailboxView } from '@/features/mailbox/MailboxView'
import { EventsView } from '@/features/events/EventsView'

export const router = createBrowserRouter([
  {
    path: '/',
    element: <Layout><WarRoom /></Layout>,
  },
  {
    path: '/projects/:projectId/war-room',
    element: <Layout><WarRoom /></Layout>,
  },
  {
    path: '/projects/:projectId/sessions',
    element: <Layout><SessionsView /></Layout>,
  },
  {
    path: '/projects/:projectId/tasks',
    element: <Layout><TasksView /></Layout>,
  },
  {
    path: '/projects/:projectId/mailbox',
    element: <Layout><MailboxView /></Layout>,
  },
  {
    path: '/projects/:projectId/events',
    element: <Layout><EventsView /></Layout>,
  },
])
