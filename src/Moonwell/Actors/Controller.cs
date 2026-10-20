namespace Moonwell
{
    /// <summary>Who controls a player slot.</summary>
    public enum Controller
    {
        /// <summary>A person.</summary>
        User,

        /// <summary>A computer.</summary>
        Computer,

        /// <summary>A rescuable player.</summary>
        Rescuable,

        /// <summary>A neutral player.</summary>
        Neutral,

        /// <summary>The creeps.</summary>
        Creep,

        /// <summary>No one.</summary>
        None,
    }
}
