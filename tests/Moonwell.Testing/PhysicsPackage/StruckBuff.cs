using System;

namespace Moonwell.Testing
{
    /// <summary>A buff the contact cases apply: it writes "&lt;Label&gt; struck" to the test log at each strike of its unit and runs the action a case gives it, as the design's Reflect.</summary>
    public sealed class StruckBuff : Buff, IMissileTarget
    {
        /// <summary>The name it logs with.</summary>
        public string Label = "buff";

        /// <summary>Run at the end of OnStruck; null for nothing.</summary>
        public Action<Impact>? Striking;

        /// <summary>Logs the strike, then runs the action.</summary>
        public void OnStruck(Impact impact)
        {
            TestLog.Lines.Add(Label + " struck");
            if (Striking != null)
            {
                Striking(impact);
            }
        }
    }
}
