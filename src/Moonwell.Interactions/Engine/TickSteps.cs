namespace Moonwell.Engine
{
    /// <summary>A due's tick in whole steps of the cycle, as Cycle.Steps gives it, kept for the tick and the step length it was worked out for, so the cycle works it out again only when one of them changes.</summary>
    internal sealed class TickSteps
    {
        /// <summary>The tick the steps were worked out for; -1 before the first.</summary>
        private float tick = -1.0f;

        /// <summary>The step length they were worked out with.</summary>
        private float length;

        /// <summary>The steps.</summary>
        private int steps;

        /// <summary>The steps a tick takes at the step length now, as Cycle.Steps gives them.</summary>
        public int Of(float seconds)
        {
            var now = Interactions.Current().Step;
            if (seconds != tick || now != length)
            {
                steps = Cycle.Steps(seconds);
                tick = seconds;
                length = now;
            }

            return steps;
        }
    }
}
