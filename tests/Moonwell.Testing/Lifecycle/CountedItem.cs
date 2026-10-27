namespace Moonwell.Testing
{
    /// <summary>An item class that counts its OnCreated and OnLeave over every item of the class; a case sets the counts to 0 first.</summary>
    public sealed class CountedItem : Item
    {
        /// <summary>How many times OnCreated has run.</summary>
        public static int Created;

        /// <summary>How many times OnLeave has run.</summary>
        public static int Left;

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
