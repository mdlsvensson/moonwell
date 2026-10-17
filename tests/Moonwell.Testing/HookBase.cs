namespace Moonwell.Testing
{
    /// <summary>A class with two reactions that do nothing; the test classes override them.</summary>
    public class HookBase
    {
        /// <summary>Does nothing; a subclass may override it.</summary>
        public virtual void OnDamaged(int amount)
        {
        }

        /// <summary>Does nothing; a subclass may override it.</summary>
        public virtual void OnKill(int victim)
        {
        }
    }
}
