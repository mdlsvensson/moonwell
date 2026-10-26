using System.Collections.Generic;

namespace Moonwell.Testing
{
    /// <summary>A component on a gizmo the cases make: it writes a line to a log when it is added and when it is removed.</summary>
    public class Part : GizmoComponent
    {
        /// <summary>The name written in its lines.</summary>
        private readonly string name;

        /// <summary>The log the lines go to.</summary>
        private readonly List<string> log;

        /// <summary>Makes a part that writes to a log under a name.</summary>
        public Part(string name, List<string> log)
        {
            this.name = name;
            this.log = log;
        }

        /// <summary>The bit mask of the reactions the part overrides, which a gizmo component has none of.</summary>
        public int Bits
        {
            get { return Mask; }
        }

        /// <summary>Writes that it was added, and how many components its gizmo has then.</summary>
        protected override void OnAdded()
        {
            log.Add(name + " added " + Gizmo.ComponentCount);
        }

        /// <summary>Writes that it was removed.</summary>
        protected override void OnRemoved()
        {
            log.Add(name + " removed");
        }
    }
}
