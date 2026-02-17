import { createContext, useContext, useState, useCallback, ReactNode } from 'react';

export interface Message {
  id: string;
  role: 'user' | 'assistant' | 'system';
  content: string;
  timestamp: number;
  skill?: string;
}

export interface Session {
  id: string;
  messages: Message[];
}

interface SessionContextType {
  currentSession: Session | null;
  setCurrentSession: (session: Session | null) => void;
  messages: Message[];
  setMessages: (messages: Message[]) => void;
  addMessage: (message: Message) => void;
  clearMessages: () => void;
  switchSession: (sessionId: string) => Promise<void>;
  createNewSession: () => Promise<void>;
  loadCurrentSession: () => Promise<void>;
}

const SessionContext = createContext<SessionContextType | undefined>(undefined);

export function SessionProvider({ children }: { children: ReactNode }) {
  const [currentSession, setCurrentSession] = useState<Session | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);

  const loadCurrentSession = useCallback(async () => {
    try {
      const response = await fetch('/api/conversations/current');
      if (!response.ok) {
        console.error('Failed to load current session');
        return;
      }
      const data = await response.json();
      if (data.id) {
        const historyMessages: Message[] = (data.messages || []).map(
          (msg: { role: string; content: string }, idx: number) => ({
            id: `history_${idx}`,
            role: msg.role as 'user' | 'assistant',
            content: msg.content,
            timestamp: Date.now() - ((data.messages?.length || 0) - idx) * 1000,
          })
        );
        setCurrentSession({ id: data.id, messages: historyMessages });
        setMessages(historyMessages);
      } else {
        setCurrentSession(null);
        setMessages([]);
      }
    } catch (error) {
      console.error('Failed to load current session:', error);
    }
  }, []);

  const switchSession = useCallback(async (sessionId: string) => {
    try {
      const response = await fetch(`/api/conversations/${sessionId}/switch`, {
        method: 'POST',
      });
      if (!response.ok) {
        console.error('Failed to switch session');
        return;
      }
      const data = await response.json();
      const historyMessages: Message[] = (data.messages || []).map(
        (msg: { role: string; content: string }, idx: number) => ({
          id: `history_${idx}`,
          role: msg.role as 'user' | 'assistant',
          content: msg.content,
          timestamp: Date.now() - ((data.messages?.length || 0) - idx) * 1000,
        })
      );
      setCurrentSession({ id: data.id, messages: historyMessages });
      setMessages(historyMessages);
    } catch (error) {
      console.error('Failed to switch session:', error);
    }
  }, []);

  const createNewSession = useCallback(async () => {
    try {
      const response = await fetch('/api/conversations', {
        method: 'POST',
      });
      if (!response.ok) {
        console.error('Failed to create new session');
        return;
      }
      const data = await response.json();
      setCurrentSession({ id: data.id, messages: [] });
      setMessages([]);
    } catch (error) {
      console.error('Failed to create new session:', error);
    }
  }, []);

  const addMessage = useCallback((message: Message) => {
    setMessages((prev) => [...prev, message]);
  }, []);

  const clearMessages = useCallback(() => {
    setMessages([]);
  }, []);

  return (
    <SessionContext.Provider
      value={{
        currentSession,
        setCurrentSession,
        messages,
        setMessages,
        addMessage,
        clearMessages,
        switchSession,
        createNewSession,
        loadCurrentSession,
      }}
    >
      {children}
    </SessionContext.Provider>
  );
}

export function useSession() {
  const context = useContext(SessionContext);
  if (context === undefined) {
    throw new Error('useSession must be used within a SessionProvider');
  }
  return context;
}
