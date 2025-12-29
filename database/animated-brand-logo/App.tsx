import React from 'react';
import AnimatedLogo from './components/AnimatedLogo';

const App: React.FC = () => {
  return (
    <div className="min-h-screen bg-black flex flex-col items-center justify-center relative overflow-hidden font-sans text-white">
      
      {/* Decorative background glow */}
      <div className="absolute top-0 left-0 w-full h-full overflow-hidden z-0 pointer-events-none">
        <div className="absolute top-[20%] left-[20%] w-[40%] h-[40%] bg-purple-900 rounded-full blur-[120px] opacity-20 animate-pulse"></div>
        <div className="absolute bottom-[20%] right-[20%] w-[40%] h-[40%] bg-blue-900 rounded-full blur-[120px] opacity-20 animate-pulse"></div>
      </div>

      <main className="z-10 flex flex-col items-center gap-16 p-8">
        
        {/* Logo Container */}
        <div className="logo-container scale-100 md:scale-125 lg:scale-150 transition-transform duration-500">
          <AnimatedLogo />
        </div>

        {/* Footer Text */}
        <div className="text-center opacity-40 text-xs tracking-[0.3em] uppercase font-light">
          <p>Digital Pulse</p>
          <p className="mt-2 text-[10px] text-gray-500">© 2024 Design</p>
        </div>

      </main>
    </div>
  );
};

export default App;