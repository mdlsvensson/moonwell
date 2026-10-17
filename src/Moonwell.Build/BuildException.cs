namespace Moonwell.Build
{
    /// <summary>An error the modder can act on; its message is the whole line the build prints.</summary>
    public sealed class BuildException : Exception
    {
        /// <summary>Creates the error with the message the build prints.</summary>
        public BuildException(string message)
            : base(message)
        {
        }
    }
}
