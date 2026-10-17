using System.Collections.Generic;

namespace Moonwell.Testing
{
    /// <summary>An owned object that logs its end, for the ownership cases.</summary>
    public class TestOwned : Owned
    {
        private readonly string name;
        private readonly List<string> log;

        /// <summary>Makes an owned object with a name that logs its end to the log.</summary>
        public TestOwned(string name, List<string> log)
        {
            this.name = name;
            this.log = log;
        }

        /// <summary>Logs the end of the object by its name.</summary>
        protected override void OnEnd()
        {
            log.Add("end " + name);
        }
    }
}
