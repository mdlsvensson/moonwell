namespace Moonwell.Testing
{
    /// <summary>A hero class that starts with a Marker and counts its OnCreated and OnLeave over every hero of the class; a case sets the counts to 0 first.</summary>
    public sealed class CountedHero : Hero
    {
        /// <summary>How many times OnCreated has run.</summary>
        public static int Created;

        /// <summary>How many times OnLeave has run.</summary>
        public static int Left;

        /// <summary>Makes a hero that starts with a Marker.</summary>
        public CountedHero()
        {
            Components.Add(new Marker());
        }

        /// <summary>Counts the run.</summary>
        protected override void OnCreated()
        {
            Created++;
        }

        /// <summary>Counts the run.</summary>
        protected override void OnLeave()
        {
            Left++;
        }
    }
}
