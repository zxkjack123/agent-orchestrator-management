import { createBrowserRouter, Navigate } from 'react-router-dom'
import { Layout } from '@/components/Layout'
import { WarRoom } from '@/features/war-room/WarRoom'
import { SessionsView } from '@/features/sessions/SessionsView'
import { TasksView } from '@/features/tasks/TasksView'
import { MailboxView } from '@/features/mailbox/MailboxView'
import { EventsView } from '@/features/events/EventsView'
import { AgentsView } from '@/features/agents/AgentsView'
import { RolesView } from '@/features/roles/RolesView'
import { DashboardView } from '@/features/dashboard/DashboardView'
import { RequestsView } from '@/features/requests/RequestsView'
import { MetricsView } from '@/features/metrics/MetricsView'
import { DoctorView } from '@/features/doctor/DoctorView'
import { TeamBriefView } from '@/features/team-brief/TeamBriefView'
import { MergeView } from '@/features/merge/MergeView'

export const router = createBrowserRouter([
  {
    path: '/',
    element: <Layout />,
    children: [
      { index: true, element: <Navigate to="." replace /> },
      {
        path: 'projects/:projectId',
        children: [
          { path: 'dashboard',   element: <DashboardView /> },
          { path: 'agents',      element: <AgentsView /> },
          { path: 'roles',       element: <RolesView /> },
          { path: 'war-room',    element: <WarRoom /> },
          { path: 'sessions',    element: <SessionsView /> },
          { path: 'tasks',       element: <TasksView /> },
          { path: 'mailbox',     element: <MailboxView /> },
          { path: 'events',      element: <EventsView /> },
          { path: 'requests',    element: <RequestsView /> },
          { path: 'team-brief',  element: <TeamBriefView /> },
          { path: 'merge',       element: <MergeView /> },
          { path: 'metrics',     element: <MetricsView /> },
          { path: 'doctor',      element: <DoctorView /> },
        ],
      },
    ],
  },
])
