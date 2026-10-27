#if DEBUG
using System;
using System.Collections.Generic;

namespace Moonwell.Testing
{
    /// <summary>A unit component with its own lines in the debug mode's panel; it can throw after adding them.</summary>
    public sealed class Shown : UnitComponent, IDebugShown
    {
        /// <summary>The lines ShowDebug adds, in order.</summary>
        public List<string> Lines = new List<string>();

        /// <summary>Whether ShowDebug throws after adding its lines.</summary>
        public bool Throws;

        /// <summary>Adds the lines, then throws when asked.</summary>
        public void ShowDebug(DebugLines lines)
        {
            for (var i = 0; i < Lines.Count; i++)
            {
                lines.Add(Lines[i]);
            }

            if (Throws)
            {
                throw new InvalidOperationException("shown boom");
            }
        }
    }
}
#endif
