import { useState, useRef, useEffect, useCallback } from "react";
import { invoke } from "@tauri-apps/api/core";
import {
  Send,
  Loader2,
  MessageSquare,
  Plus,
  Trash2,
  History,
  Sparkles,
  ChevronDown,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { ScrollArea } from "@/components/ui/scroll-area";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { cn } from "@/lib/utils";
import { useChatStore, type ChatSession } from "@/stores/chatStore";
import { toast } from "sonner";

interface CompletionResponse {
  success: boolean;
  content: string | null;
  error: string | null;
  model_used: string | null;
}

interface AIChatProps {
  context?: ChatSession["context"];
  trigger?: React.ReactNode;
}

export function AIChat({ context, trigger }: AIChatProps) {
  const [open, setOpen] = useState(false);
  const [input, setInput] = useState("");
  const [loading, setLoading] = useState(false);
  const [historyOpen, setHistoryOpen] = useState(false);
  const [providers, setProviders] = useState<string[]>([]);
  const [selectedProvider, setSelectedProvider] = useState<string | null>(null);
  const scrollRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLTextAreaElement>(null);

  // Fetch available providers on mount
  useEffect(() => {
    invoke<string[]>("get_available_ai_providers")
      .then((list) => {
        setProviders(list);
        // Don't auto-select - let it use default (first available)
      })
      .catch(console.error);
  }, []);

  const {
    sessions,
    activeSessionId,
    createSession,
    deleteSession,
    setActiveSession,
    getActiveSession,
    addMessage,
  } = useChatStore();

  const activeSession = getActiveSession();

  // Auto-scroll to bottom when messages change
  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [activeSession?.messages]);

  // Create new session if none exists when opening
  useEffect(() => {
    if (open && !activeSessionId) {
      createSession(context);
    }
  }, [open, activeSessionId, createSession, context]);

  const handleSend = useCallback(async () => {
    if (!input.trim() || loading) return;

    let sessionId = activeSessionId;

    // Create session if needed
    if (!sessionId) {
      sessionId = createSession(context);
    }

    const userMessage = input.trim();
    setInput("");
    setLoading(true);

    // Add user message
    addMessage(sessionId, {
      role: "user",
      content: userMessage,
    });

    try {
      // Build prompt with context
      let prompt = userMessage;
      const session = sessions.find((s) => s.id === sessionId);

      if (session?.context?.type === "email" && session.context.emailSubject) {
        prompt = `Context: You are helping the user with an email.\nEmail Subject: ${session.context.emailSubject}\n\nUser question: ${userMessage}`;
      }

      // Build system prompt
      let systemPrompt = "You are a helpful email assistant. Be concise and helpful.";

      // Include conversation history for context
      if (session && session.messages.length > 0) {
        const history = session.messages
          .slice(-6) // Last 6 messages for context
          .map((m) => `${m.role === "user" ? "User" : "Assistant"}: ${m.content}`)
          .join("\n\n");

        systemPrompt += `\n\nPrevious conversation:\n${history}`;
      }

      const response = await invoke<CompletionResponse>("ai_complete", {
        request: {
          prompt,
          system_prompt: systemPrompt,
          max_tokens: 1000,
          provider_name: selectedProvider,
        },
      });

      if (response.success && response.content) {
        addMessage(sessionId, {
          role: "assistant",
          content: response.content,
          modelUsed: response.model_used || undefined,
        });
      } else {
        toast.error(response.error || "Failed to get AI response");
      }
    } catch (err) {
      toast.error(`AI error: ${err}`);
    } finally {
      setLoading(false);
      inputRef.current?.focus();
    }
  }, [input, loading, activeSessionId, context, createSession, addMessage, sessions, selectedProvider]);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  const handleNewChat = () => {
    createSession(context);
    setHistoryOpen(false);
  };

  const handleSelectSession = (sessionId: string) => {
    setActiveSession(sessionId);
    setHistoryOpen(false);
  };

  const handleDeleteSession = (e: React.MouseEvent, sessionId: string) => {
    e.stopPropagation();
    deleteSession(sessionId);
  };

  const formatTime = (timestamp: number) => {
    const date = new Date(timestamp);
    const now = new Date();
    const diff = now.getTime() - date.getTime();

    if (diff < 60000) return "Just now";
    if (diff < 3600000) return `${Math.floor(diff / 60000)}m ago`;
    if (diff < 86400000) return `${Math.floor(diff / 3600000)}h ago`;
    return date.toLocaleDateString();
  };

  const defaultTrigger = (
    <Button variant="outline" size="icon" className="rounded-full">
      <MessageSquare className="h-4 w-4" />
    </Button>
  );

  return (
    <Sheet open={open} onOpenChange={setOpen}>
      <SheetTrigger asChild>{trigger || defaultTrigger}</SheetTrigger>
      <SheetContent className="w-[400px] sm:w-[540px] flex flex-col p-0">
        <SheetHeader className="px-4 py-3 border-b flex-shrink-0">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <SheetTitle className="flex items-center gap-2">
                <Sparkles className="h-5 w-5" />
                AI Chat
              </SheetTitle>
              {providers.length > 0 && (
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <Button variant="outline" size="sm" className="h-7 text-xs gap-1">
                      {selectedProvider || "Auto"}
                      <ChevronDown className="h-3 w-3" />
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="start">
                    <DropdownMenuItem onClick={() => setSelectedProvider(null)}>
                      Auto (default)
                    </DropdownMenuItem>
                    {providers.map((provider) => (
                      <DropdownMenuItem
                        key={provider}
                        onClick={() => setSelectedProvider(provider)}
                      >
                        {provider}
                      </DropdownMenuItem>
                    ))}
                  </DropdownMenuContent>
                </DropdownMenu>
              )}
            </div>
            <div className="flex items-center gap-1">
              <Popover open={historyOpen} onOpenChange={setHistoryOpen}>
                <PopoverTrigger asChild>
                  <Button variant="ghost" size="icon">
                    <History className="h-4 w-4" />
                  </Button>
                </PopoverTrigger>
                <PopoverContent className="w-80 p-2" align="end">
                  <div className="space-y-1">
                    <div className="flex items-center justify-between px-2 py-1">
                      <span className="text-xs font-medium text-muted-foreground">
                        Chat History
                      </span>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={handleNewChat}
                        className="h-7 text-xs"
                      >
                        <Plus className="h-3 w-3 mr-1" />
                        New
                      </Button>
                    </div>
                    <ScrollArea className="max-h-64">
                      {sessions.length === 0 ? (
                        <p className="text-sm text-muted-foreground px-2 py-4 text-center">
                          No chat history
                        </p>
                      ) : (
                        sessions.map((session) => (
                          <button
                            key={session.id}
                            onClick={() => handleSelectSession(session.id)}
                            className={cn(
                              "w-full flex items-center justify-between rounded-md px-2 py-2 text-left hover:bg-muted group",
                              activeSessionId === session.id && "bg-muted"
                            )}
                          >
                            <div className="min-w-0 flex-1">
                              <p className="text-sm font-medium truncate">
                                {session.title}
                              </p>
                              <p className="text-xs text-muted-foreground">
                                {session.messages.length} messages •{" "}
                                {formatTime(session.updatedAt)}
                              </p>
                            </div>
                            <Button
                              variant="ghost"
                              size="icon"
                              className="h-6 w-6 opacity-0 group-hover:opacity-100"
                              onClick={(e) => handleDeleteSession(e, session.id)}
                            >
                              <Trash2 className="h-3 w-3" />
                            </Button>
                          </button>
                        ))
                      )}
                    </ScrollArea>
                  </div>
                </PopoverContent>
              </Popover>
              <Button
                variant="ghost"
                size="icon"
                onClick={handleNewChat}
              >
                <Plus className="h-4 w-4" />
              </Button>
            </div>
          </div>
        </SheetHeader>

        {/* Messages */}
        <ScrollArea ref={scrollRef} className="flex-1 px-4 py-4">
          {activeSession?.messages.length === 0 ? (
            <div className="flex flex-col items-center justify-center h-full text-center py-12">
              <MessageSquare className="h-12 w-12 text-muted-foreground mb-4" />
              <h3 className="font-medium text-lg">Start a conversation</h3>
              <p className="text-sm text-muted-foreground max-w-xs mt-1">
                Ask questions about your emails, get help drafting responses, or
                explore your inbox with AI assistance.
              </p>
            </div>
          ) : (
            <div className="space-y-4">
              {activeSession?.messages.map((message) => (
                <div
                  key={message.id}
                  className={cn(
                    "flex",
                    message.role === "user" ? "justify-end" : "justify-start"
                  )}
                >
                  <div
                    className={cn(
                      "max-w-[85%] rounded-lg px-3 py-2",
                      message.role === "user"
                        ? "bg-primary text-primary-foreground"
                        : "bg-muted"
                    )}
                  >
                    <p className="text-sm whitespace-pre-wrap">{message.content}</p>
                    {message.modelUsed && (
                      <p className="text-xs opacity-60 mt-1">
                        {message.modelUsed}
                      </p>
                    )}
                  </div>
                </div>
              ))}
              {loading && (
                <div className="flex justify-start">
                  <div className="bg-muted rounded-lg px-3 py-2">
                    <Loader2 className="h-4 w-4 animate-spin" />
                  </div>
                </div>
              )}
            </div>
          )}
        </ScrollArea>

        {/* Input */}
        <div className="flex-shrink-0 border-t p-4">
          <div className="flex gap-2">
            <Textarea
              ref={inputRef}
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={handleKeyDown}
              placeholder="Ask anything about your emails..."
              className="min-h-[60px] max-h-[120px] resize-none"
              disabled={loading}
            />
            <Button
              onClick={handleSend}
              disabled={!input.trim() || loading}
              className="self-end"
            >
              {loading ? (
                <Loader2 className="h-4 w-4 animate-spin" />
              ) : (
                <Send className="h-4 w-4" />
              )}
            </Button>
          </div>
        </div>
      </SheetContent>
    </Sheet>
  );
}
