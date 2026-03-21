import { useState, useRef } from 'react';

interface Props {
  onTranscript: (text: string) => void;
  disabled?: boolean;
}

export default function VoiceInput({ onTranscript, disabled }: Props) {
  const [listening, setListening] = useState(false);
  const recognitionRef = useRef<any>(null);

  const toggle = () => {
    if (listening) {
      recognitionRef.current?.stop();
      setListening(false);
      return;
    }

    const SpeechRecognition = (window as any).SpeechRecognition || (window as any).webkitSpeechRecognition;
    if (!SpeechRecognition) {
      alert('Speech recognition not supported in this browser');
      return;
    }

    const recognition = new SpeechRecognition();
    recognition.continuous = true;
    recognition.interimResults = true;
    recognition.lang = localStorage.getItem('lang') === 'zh' ? 'zh-TW' : 'en-US';

    recognition.onresult = (event: any) => {
      let transcript = '';
      for (let i = event.resultIndex; i < event.results.length; i++) {
        transcript += event.results[i][0].transcript;
      }
      if (event.results[event.results.length - 1].isFinal) {
        onTranscript(transcript);
      }
    };

    recognition.onerror = () => setListening(false);
    recognition.onend = () => setListening(false);

    recognitionRef.current = recognition;
    recognition.start();
    setListening(true);
  };

  // Don't render if not supported
  if (typeof window !== 'undefined' && !((window as any).SpeechRecognition || (window as any).webkitSpeechRecognition)) {
    return null;
  }

  return (
    <button
      onClick={toggle}
      disabled={disabled}
      className={`p-2 rounded-lg transition-colors ${
        listening
          ? 'bg-red-500 text-white animate-pulse'
          : 'hover:bg-gray-600 text-gray-400'
      }`}
      title={listening ? 'Stop recording' : 'Voice input'}
    >
      🎤
    </button>
  );
}
