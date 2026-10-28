using System.Collections.Generic;

namespace Moonwell.Testing
{
    /// <summary>A gizmo that lists three parts in its constructor, and writes a line to a log when its OnCreated runs, with how many components it has then.</summary>
    public class Fitted : Gizmo
    {
        /// <summary>The log the lines go to.</summary>
        private readonly List<string> log;

        /// <summary>Makes a gizmo that lists the parts a, b and c, which write to the log.</summary>
        public Fitted(List<string> log)
        {
            this.log = log;
            Components.Add(new Part("a", log), new Part("b", log), new Part("c", log));
        }

        /// <summary>Writes that it was created, and how many components it has then.</summary>
        protected override void OnCreated()
        {
            log.Add("created " + ComponentCount);
        }
    }
}
