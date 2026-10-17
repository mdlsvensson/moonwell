using System;

namespace Moonwell.Testing
{
    /// <summary>A named check that runs against a fresh world.</summary>
    public sealed class Case
    {
        /// <summary>The name the output and the .NET test show.</summary>
        public string Name = "";

        /// <summary>The check; it throws when it fails.</summary>
        public Action<FakeWorld> Run = _ => { };
    }
}
