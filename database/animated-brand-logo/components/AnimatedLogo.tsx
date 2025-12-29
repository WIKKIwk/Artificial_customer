import React from 'react';

const AnimatedLogo: React.FC = () => {
  // Configuration for the geometric logo
  // Increased columns to 15 for a smoother star shape
  const numCols = 15; 
  const width = 300;
  const height = 300;
  const centerX = width / 2;
  const centerY = height / 2;
  const maxRadius = 130; // Slightly larger outer circle
  
  // Bar styling
  const barWidth = 12; // Slightly thinner to fit more columns
  const barGap = 6;
  const cornerRadius = 6;

  // Generate bars
  const bars = [];

  for (let i = 0; i < numCols; i++) {
    // Calculate distance from center column (0 is center)
    // Map i (0..14) to xIndex (-7..7)
    const xIndex = i - Math.floor(numCols / 2); 
    
    // Physical X position
    const xPos = centerX + xIndex * (barWidth + barGap) - (barWidth / 2);
    
    // 1. Calculate Outer Height Limit (Circle Shape)
    // y^2 + x^2 = r^2  => y = sqrt(r^2 - x^2)
    const xRel = xIndex * (barWidth + barGap);
    const outerHalfHeight = Math.sqrt(Math.max(0, maxRadius * maxRadius - xRel * xRel));

    // 2. Calculate Inner Gap (Gemini Star / Astroid Shape)
    // The Gemini star is concave. To achieve this, we use a higher power curve.
    // We want a sharp spike vertically.
    
    const maxGapHalfHeight = 65; // Taller gap in the center
    
    // Using power of 2.8 makes the curve concave (curved inwards) -> Sharp Star
    // We clamp the calc so negative values don't break logic
    let starCurve = Math.pow(1 - Math.min(1, Math.abs(xRel) / 100), 2.8);
    
    // If we are at the very edge, force gap to 0 to close the circle shape naturally
    if (Math.abs(xRel) > 90) starCurve = 0;

    const innerGapHalfHeight = maxGapHalfHeight * starCurve;

    // Actual height of the top and bottom bars
    let barHeight = outerHalfHeight - innerGapHalfHeight;
    
    // Ensure bar doesn't disappear completely or invert
    if (barHeight < 0) barHeight = 0;

    // Animation logic
    // We make the animation faster at the edges and slower in center for a "breathing" heart effect
    const delay = Math.abs(xIndex) * 0.08; 
    
    const animClass = xIndex % 2 === 0 ? 'animate-bar' : 'animate-bar-reverse';

    // Only render if there is height
    if (barHeight > 1) {
      // TOP BAR
      bars.push(
        <rect
          key={`top-${i}`}
          x={xPos}
          y={centerY - innerGapHalfHeight - barHeight}
          width={barWidth}
          height={barHeight}
          rx={cornerRadius}
          ry={cornerRadius}
          fill="url(#mainGradient)"
          className={animClass}
          style={{ 
            transformBox: 'fill-box', 
            transformOrigin: 'bottom center', // Anchors to the Star shape
            animationDelay: `${delay}s`
          }}
        />
      );

      // BOTTOM BAR
      bars.push(
        <rect
          key={`bottom-${i}`}
          x={xPos}
          y={centerY + innerGapHalfHeight}
          width={barWidth}
          height={barHeight}
          rx={cornerRadius}
          ry={cornerRadius}
          fill="url(#mainGradient)"
          className={animClass}
          style={{ 
            transformBox: 'fill-box', 
            transformOrigin: 'top center', // Anchors to the Star shape
            animationDelay: `${delay}s`
          }}
        />
      );
    }
  }

  return (
    <div className="relative group cursor-pointer">
      {/* Container for the logo */}
      <div className="w-[300px] h-[300px] relative transition-all duration-700 hover:scale-105">
        <svg 
          viewBox="0 0 300 300" 
          className="w-full h-full drop-shadow-[0_0_30px_rgba(168,85,247,0.4)]"
          xmlns="http://www.w3.org/2000/svg"
          style={{ animation: 'hue-shift 12s infinite linear' }}
        >
          <defs>
            {/* 
              Gemini-inspired Gradient 
              Deep Blue -> Purple -> Pink -> Amber
            */}
            <linearGradient id="mainGradient" x1="0%" y1="100%" x2="100%" y2="0%">
              <stop offset="0%" stopColor="#2563eb" />   {/* Blue 600 */}
              <stop offset="30%" stopColor="#7c3aed" />  {/* Violet 600 */}
              <stop offset="60%" stopColor="#db2777" />  {/* Pink 600 */}
              <stop offset="100%" stopColor="#fbbf24" /> {/* Amber 400 */}
            </linearGradient>

            <filter id="glow" x="-20%" y="-20%" width="140%" height="140%">
              <feGaussianBlur stdDeviation="3" result="blur" />
              <feComposite in="SourceGraphic" in2="blur" operator="over" />
            </filter>
          </defs>

          {/* Render bars */}
          <g>
             {bars}
          </g>
        </svg>
      </div>
    </div>
  );
};

export default AnimatedLogo;