namespace Moonwell.Testing
{
    /// <summary>A hero that starts with a Tally, and logs when it enters and when it leaves.</summary>
    public class TestHero : Hero
    {
        /// <summary>Makes a hero that starts with a Tally.</summary>
        public TestHero()
        {
            Components.Add(new Tally());
        }

        /// <summary>Logs enter.</summary>
        protected internal override void OnEnter()
        {
            TestLog.Lines.Add("enter");
        }

        /// <summary>Logs leave.</summary>
        protected internal override void OnLeave()
        {
            TestLog.Lines.Add("leave");
        }
    }
}
