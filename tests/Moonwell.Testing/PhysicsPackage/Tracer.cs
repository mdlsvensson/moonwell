using System.Collections.Generic;

namespace Moonwell.Testing
{
    /// <summary>A missile component that writes its reactions' names, and the owner current in them, to a log a case gives it.</summary>
    public sealed class Tracer : MissileComponent
    {
        /// <summary>Where it writes.</summary>
        private readonly List<string> log;

        /// <summary>The owner that was current when OnImpact last ran.</summary>
        public Scope? ImpactedWith;

        /// <summary>The owner that was current when OnRest last ran.</summary>
        public Scope? RestedWith;

        /// <summary>Makes a tracer that writes to a log.</summary>
        public Tracer(List<string> log)
        {
            this.log = log;
        }

        /// <summary>Writes its name and keeps the current owner.</summary>
        public override void OnImpact(Impact impact)
        {
            ImpactedWith = Scope.Current;
            log.Add("tracer.impact");
        }

        /// <summary>Writes its name and keeps the current owner.</summary>
        public override void OnRest()
        {
            RestedWith = Scope.Current;
            log.Add("tracer.rest");
        }
    }
}
