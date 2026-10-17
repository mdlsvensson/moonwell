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
        protected override void OnEnter()
        {
            TestLog.Lines.Add("enter");
        }

        /// <summary>Logs leave.</summary>
        protected override void OnLeave()
        {
            TestLog.Lines.Add("leave");
        }

        /// <summary>Logs that the hero got an item.</summary>
        public override void OnItemPickedUp(Item item)
        {
            TestLog.Lines.Add("hero got orb");
        }

        /// <summary>Logs that the hero dropped an item.</summary>
        public override void OnItemDropped(Item item)
        {
            TestLog.Lines.Add("hero dropped orb");
        }
    }
}
