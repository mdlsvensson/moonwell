using System.Collections.Generic;

namespace Moonwell.Testing
{
    /// <summary>An owner that logs its end, for the ownership cases.</summary>
    public class TestOwner : Owner
    {
        private readonly string name;
        private readonly List<string> log;

        /// <summary>Makes an owner with a name that logs its end to the log.</summary>
        public TestOwner(string name, List<string> log)
        {
            this.name = name;
            this.log = log;
        }

        /// <summary>Logs the end of the owner by its name.</summary>
        protected override void OnEnd()
        {
            log.Add("end " + name);
        }
    }
}
