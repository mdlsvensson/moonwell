namespace Moonwell.Testing
{
    /// <summary>An item that keeps the carrier it sees when it is picked up and when it is dropped.</summary>
    public class CarrierWatch : Item
    {
        /// <summary>The carrier the item had inside OnPickedUp.</summary>
        public Unit? CarrierAtPickup;

        /// <summary>The carrier the item had inside OnDropped.</summary>
        public Unit? CarrierAtDrop;

        /// <summary>Whether OnDropped has run.</summary>
        public bool DropSeen;

        /// <summary>Keeps the carrier seen.</summary>
        public override void OnPickedUp(Unit by)
        {
            CarrierAtPickup = Carrier;
        }

        /// <summary>Keeps the carrier seen.</summary>
        public override void OnDropped(Unit by)
        {
            CarrierAtDrop = Carrier;
            DropSeen = true;
        }
    }
}
