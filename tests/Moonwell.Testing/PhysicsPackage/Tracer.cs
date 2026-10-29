using System.Collections.Generic;

namespace Moonwell.Testing
{
    /// <summary>A missile component that writes its reactions' names to a log a case gives it.</summary>
    public sealed class Tracer : MissileComponent
    {
        /// <summary>Where it writes.</summary>
        private readonly List<string> log;

        /// <summary>Makes a tracer that writes to a log.</summary>
        public Tracer(List<string> log)
        {
            this.log = log;
        }

        /// <summary>Writes its name.</summary>
        public override void OnImpact(Impact impact)
        {
            log.Add("tracer.impact");
        }

        /// <summary>Writes its name.</summary>
        public override void OnRest()
        {
            log.Add("tracer.rest");
        }
    }
}
