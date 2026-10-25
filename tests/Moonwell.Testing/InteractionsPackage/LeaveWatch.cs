using System;

namespace Moonwell.Testing
{
    /// <summary>A unit that runs the action a case gives it in its OnLeave.</summary>
    public sealed class LeaveWatch : Unit
    {
        /// <summary>An action OnLeave runs; null for none.</summary>
        public Action? Leaving;

        /// <summary>Runs the action.</summary>
        protected override void OnLeave()
        {
            if (Leaving != null)
            {
                Leaving();
            }
        }
    }
}
