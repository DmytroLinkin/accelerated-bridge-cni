package manager

import (
	"errors"
	"net"

	"github.com/containernetworking/plugins/pkg/ns"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/vishvananda/netlink"

	"github.com/DmytroLinkin/accelerated-bridge-cni/pkg/types"
	"github.com/DmytroLinkin/accelerated-bridge-cni/pkg/types/mocks"
)

// Fake NS - implements ns.NetNS interface
type fakeNetNS struct {
	closed bool
	fd     uintptr
	path   string
}

func (f *fakeNetNS) Do(toRun func(ns.NetNS) error) error {
	return toRun(f)
}

func (f *fakeNetNS) Set() error {
	return nil
}

func (f *fakeNetNS) Path() string {
	return f.path
}

func (f *fakeNetNS) Fd() uintptr {
	return f.fd
}

func (f *fakeNetNS) Close() error {
	f.closed = true
	return nil
}

func newFakeNs() ns.NetNS {
	return &fakeNetNS{
		closed: false,
		fd:     17,
		path:   "/proc/4123/ns/net",
	}
}

// FakeLink is a dummy netlink struct used during testing
type FakeLink struct {
	netlink.LinkAttrs
}

func (l *FakeLink) Attrs() *netlink.LinkAttrs {
	return &l.LinkAttrs
}

func (l *FakeLink) Type() string {
	return "FakeLink"
}

var _ = Describe("Manager", func() {
	var (
		t GinkgoTInterface
	)
	BeforeEach(func() {
		t = GinkgoT()
	})

	Context("Checking SetupVF function", func() {
		var (
			podifName string
			contID    string
			netconf   *types.NetConf
		)

		BeforeEach(func() {
			podifName = "net1"
			contID = "dummycid"
			netconf = &types.NetConf{
				Master:      "enp175s0f1",
				DeviceID:    "0000:af:06.0",
				VFID:        0,
				ContIFNames: "net1",
				OrigVfState: types.VfState{
					HostIFName: "enp175s6",
				},
			}
			t = GinkgoT()
		})

		It("Assuming existing interface", func() {
			targetNetNS := newFakeNs()
			mocked := &mocks.NetlinkManager{}
			fakeMac, err := net.ParseMAC("6e:16:06:0e:b7:e9")

			Expect(err).NotTo(HaveOccurred())

			fakeLink := &FakeLink{netlink.LinkAttrs{
				Index:        1000,
				Name:         "dummylink",
				HardwareAddr: fakeMac,
			}}

			mocked.On("LinkByName", mock.AnythingOfType("string")).Return(fakeLink, nil)
			mocked.On("LinkSetDown", fakeLink).Return(nil)
			mocked.On("LinkSetName", fakeLink, mock.Anything).Return(nil)
			mocked.On("LinkSetNsFd", fakeLink, mock.AnythingOfType("int")).Return(nil)
			mocked.On("LinkSetUp", fakeLink).Return(nil)
			m := manager{nLink: mocked}
			macAddr, err := m.SetupVF(netconf, podifName, contID, targetNetNS)
			Expect(err).NotTo(HaveOccurred())
			Expect(macAddr).To(Equal("6e:16:06:0e:b7:e9"))
			mocked.AssertExpectations(t)
		})
		It("Setting mac address", func() {
			targetNetNS := newFakeNs()
			mocked := &mocks.NetlinkManager{}
			fakeMac, err := net.ParseMAC("6e:16:06:0e:b7:e9")
			Expect(err).NotTo(HaveOccurred())
			netconf.MAC = "e4:11:22:33:44:55"
			expMac, err := net.ParseMAC(netconf.MAC)
			Expect(err).NotTo(HaveOccurred())

			fakeLink := &FakeLink{netlink.LinkAttrs{
				Index:        1000,
				Name:         "dummylink",
				HardwareAddr: fakeMac,
			}}

			mocked.On("LinkByName", mock.AnythingOfType("string")).Return(fakeLink, nil)
			mocked.On("LinkSetDown", fakeLink).Return(nil)
			mocked.On("LinkSetName", fakeLink, mock.Anything).Return(nil)
			mocked.On("LinkSetHardwareAddr", fakeLink, expMac).Return(nil)
			mocked.On("LinkSetNsFd", fakeLink, mock.AnythingOfType("int")).Return(nil)
			mocked.On("LinkSetUp", fakeLink).Return(nil)
			m := manager{nLink: mocked}
			macAddr, err := m.SetupVF(netconf, podifName, contID, targetNetNS)
			Expect(err).NotTo(HaveOccurred())
			Expect(macAddr).To(Equal(netconf.MAC))
			mocked.AssertExpectations(t)
		})
	})

	Context("Checking ReleaseVF function", func() {
		var (
			podifName string
			contID    string
			netconf   *types.NetConf
		)

		BeforeEach(func() {
			podifName = "net1"
			contID = "dummycid"
			netconf = &types.NetConf{
				Master:      "enp175s0f1",
				DeviceID:    "0000:af:06.0",
				VFID:        0,
				ContIFNames: "net1",
				OrigVfState: types.VfState{
					HostIFName: "enp175s6",
				},
			}
		})
		It("Assuming existing interface", func() {
			targetNetNS := newFakeNs()
			mocked := &mocks.NetlinkManager{}
			fakeLink := &FakeLink{netlink.LinkAttrs{Index: 1000, Name: "dummylink"}}

			mocked.On("LinkByName", netconf.ContIFNames).Return(fakeLink, nil)
			mocked.On("LinkSetDown", fakeLink).Return(nil)
			mocked.On("LinkSetName", fakeLink, netconf.OrigVfState.HostIFName).Return(nil)
			mocked.On("LinkSetNsFd", fakeLink, mock.AnythingOfType("int")).Return(nil)
			m := manager{nLink: mocked}
			err := m.ReleaseVF(netconf, podifName, contID, targetNetNS)
			Expect(err).NotTo(HaveOccurred())
			mocked.AssertExpectations(t)
		})
	})
	Context("Checking ReleaseVF function - restore config", func() {
		var (
			podifName string
			contID    string
			netconf   *types.NetConf
		)

		BeforeEach(func() {
			podifName = "net1"
			contID = "dummycid"
			netconf = &types.NetConf{
				Master:      "enp175s0f1",
				DeviceID:    "0000:af:06.0",
				VFID:        0,
				MAC:         "aa:f3:8d:65:1b:d4",
				ContIFNames: "net1",
				OrigVfState: types.VfState{
					HostIFName:   "enp175s6",
					EffectiveMAC: "c6:c8:7f:1f:21:90",
				},
			}
		})
		It("Restores Effective MAC address when provided in netconf", func() {
			targetNetNS := newFakeNs()
			mocked := &mocks.NetlinkManager{}
			fakeLink := &FakeLink{netlink.LinkAttrs{Index: 1000, Name: "dummylink"}}

			mocked.On("LinkByName", netconf.ContIFNames).Return(fakeLink, nil)
			mocked.On("LinkSetDown", fakeLink).Return(nil)
			mocked.On("LinkSetName", fakeLink, netconf.OrigVfState.HostIFName).Return(nil)
			mocked.On("LinkSetNsFd", fakeLink, mock.AnythingOfType("int")).Return(nil)
			origEffMac, err := net.ParseMAC(netconf.OrigVfState.EffectiveMAC)
			Expect(err).NotTo(HaveOccurred())
			mocked.On("LinkSetHardwareAddr", fakeLink, origEffMac).Return(nil)
			m := manager{nLink: mocked}
			err = m.ReleaseVF(netconf, podifName, contID, targetNetNS)
			Expect(err).NotTo(HaveOccurred())
			mocked.AssertExpectations(t)
		})
	})
	Context("Checking ResetVFConfig function - restore config no user params", func() {
		var (
			netconf *types.NetConf
		)

		BeforeEach(func() {
			netconf = &types.NetConf{
				Master:      "enp175s0f1",
				DeviceID:    "0000:af:06.0",
				VFID:        0,
				ContIFNames: "net1",
				OrigVfState: types.VfState{
					HostIFName: "enp175s6",
				},
			}
		})
		It("Does not change VF config if it wasnt requested to be changed in netconf", func() {
			mocked := &mocks.NetlinkManager{}
			fakeLink := &FakeLink{netlink.LinkAttrs{Index: 1000, Name: "dummylink"}}

			mocked.On("LinkByName", netconf.Master).Return(fakeLink, nil)
			m := manager{nLink: mocked}
			err := m.ResetVFConfig(netconf)
			Expect(err).NotTo(HaveOccurred())
			mocked.AssertExpectations(t)
		})
	})

	Context("Checking ResetVFConfig function - restore config with user params", func() {
		var (
			netconf *types.NetConf
		)

		BeforeEach(func() {
			vlan := 6
			vlanQos := 3
			maxTxRate := 4000
			minTxRate := 1000

			netconf = &types.NetConf{
				Master:      "enp175s0f1",
				DeviceID:    "0000:af:06.0",
				VFID:        3,
				ContIFNames: "net1",
				MAC:         "d2:fc:22:a7:0d:e8",
				Vlan:        &vlan,
				VlanQoS:     &vlanQos,
				SpoofChk:    "on",
				MaxTxRate:   &maxTxRate,
				MinTxRate:   &minTxRate,
				Trust:       "on",
				LinkState:   "enable",
				OrigVfState: types.VfState{
					HostIFName:   "enp175s6",
					SpoofChk:     false,
					AdminMAC:     "aa:f3:8d:65:1b:d4",
					EffectiveMAC: "aa:f3:8d:65:1b:d4",
					Vlan:         1,
					VlanQoS:      1,
					MinTxRate:    0,
					MaxTxRate:    0,
					LinkState:    2, // disable
				},
			}
		})
		It("Restores original VF configurations", func() {
			mocked := &mocks.NetlinkManager{}
			fakeLink := &FakeLink{netlink.LinkAttrs{Index: 1000, Name: "dummylink"}}

			mocked.On("LinkByName", netconf.Master).Return(fakeLink, nil)
			mocked.On("LinkSetVfVlanQos", fakeLink, netconf.VFID, netconf.OrigVfState.Vlan,
				netconf.OrigVfState.VlanQoS).Return(nil)
			mocked.On("LinkSetVfSpoofchk", fakeLink, netconf.VFID, netconf.OrigVfState.SpoofChk).Return(nil)
			origMac, err := net.ParseMAC(netconf.OrigVfState.AdminMAC)
			Expect(err).NotTo(HaveOccurred())
			mocked.On("LinkSetVfHardwareAddr", fakeLink, netconf.VFID, origMac).Return(nil)
			mocked.On("LinkSetVfTrust", fakeLink, netconf.VFID, false).Return(nil)
			mocked.On("LinkSetVfRate", fakeLink, netconf.VFID, netconf.OrigVfState.MinTxRate,
				netconf.OrigVfState.MaxTxRate).Return(nil)
			mocked.On("LinkSetVfState", fakeLink, netconf.VFID, netconf.OrigVfState.LinkState).Return(nil)

			m := manager{nLink: mocked}
			err = m.ResetVFConfig(netconf)
			Expect(err).NotTo(HaveOccurred())
			mocked.AssertExpectations(t)
		})
	})
	Context("Checking AttachRepresentor function", func() {
		var (
			netconf *types.NetConf
		)

		BeforeEach(func() {
			netconf = &types.NetConf{
				Representor: "dummylink",
				Master:      "enp175s0f1",
				DeviceID:    "0000:af:06.0",
				VFID:        0,
			}
			// Mute logger
			zerolog.SetGlobalLevel(zerolog.Disabled)
		})
		It("Attaching dummy link to the bridge (success)", func() {
			mockedNl := &mocks.NetlinkManager{}
			mockedSr := &mocks.Sriovnet{}
			fakeBridge := &netlink.Bridge{LinkAttrs: netlink.LinkAttrs{Index: 1000, Name: "cni0"}}
			fakeLink := &FakeLink{netlink.LinkAttrs{
				Name:        netconf.Representor,
				MasterIndex: 0,
			}}

			mockedNl.On("LinkByName", netconf.Bridge).Return(fakeBridge, nil)
			mockedNl.On("LinkByName", netconf.Representor).Return(fakeLink, nil)
			mockedSr.On("GetVfRepresentor", netconf.Master, netconf.VFID).Return(fakeLink.Name, nil)
			mockedNl.On("LinkSetUp", fakeLink).Return(nil)
			mockedNl.On("LinkSetMaster", fakeLink, fakeBridge).Run(func(args mock.Arguments) {
				link := args.Get(0).(netlink.Link)
				bridge := args.Get(1).(netlink.Link)
				link.Attrs().MasterIndex = bridge.Attrs().Index
			}).Return(nil)

			m := manager{nLink: mockedNl, sriov: mockedSr}
			err := m.AttachRepresentor(netconf)
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeLink.Attrs().MasterIndex).To(Equal(fakeBridge.Attrs().Index))
			mockedNl.AssertExpectations(t)
			mockedSr.AssertExpectations(t)
		})
		It("Attaching dummy link to the bridge (failure)", func() {
			mockedNl := &mocks.NetlinkManager{}
			mockedSr := &mocks.Sriovnet{}
			fakeBridge := &netlink.Bridge{LinkAttrs: netlink.LinkAttrs{Name: "cni0"}}
			fakeLink := &FakeLink{netlink.LinkAttrs{
				Name:        netconf.Representor,
				MasterIndex: 0,
			}}

			mockedNl.On("LinkByName", netconf.Bridge).Return(fakeBridge, nil)
			mockedNl.On("LinkByName", netconf.Representor).Return(fakeLink, nil)
			mockedSr.On("GetVfRepresentor", netconf.Master, netconf.VFID).Return(fakeLink.Name, nil)
			mockedNl.On("LinkSetUp", fakeLink).Return(nil)
			mockedNl.On("LinkSetMaster", fakeLink, fakeBridge).Return(errors.New("some error"))

			m := manager{nLink: mockedNl, sriov: mockedSr}
			err := m.AttachRepresentor(netconf)
			Expect(err).To(HaveOccurred())
			mockedNl.AssertExpectations(t)
			mockedSr.AssertExpectations(t)
		})
	})
	Context("Checking DetachRepresentor function", func() {
		var (
			netconf *types.NetConf
		)

		BeforeEach(func() {
			netconf = &types.NetConf{
				Representor: "dummylink",
				Master:      "enp175s0f1",
				DeviceID:    "0000:af:06.0",
				VFID:        0,
			}
			// Mute logger
			zerolog.SetGlobalLevel(zerolog.Disabled)
		})
		It("Detaching dummy link from the bridge (success)", func() {
			mocked := &mocks.NetlinkManager{}
			fakeLink := &FakeLink{netlink.LinkAttrs{
				Name:        netconf.Representor,
				MasterIndex: 1000,
			}}

			mocked.On("LinkByName", netconf.Representor).Return(fakeLink, nil)
			mocked.On("LinkSetDown", fakeLink).Return(nil)
			mocked.On("LinkSetNoMaster", fakeLink).Run(func(args mock.Arguments) {
				link := args.Get(0).(netlink.Link)
				link.Attrs().MasterIndex = 0
			}).Return(nil)

			m := manager{nLink: mocked}
			err := m.DetachRepresentor(netconf)
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeLink.Attrs().MasterIndex).To(Equal(0))
			mocked.AssertExpectations(t)
		})
		It("Detaching dummy link from the bridge (failure)", func() {
			mocked := &mocks.NetlinkManager{}
			fakeLink := &FakeLink{netlink.LinkAttrs{
				Name:        netconf.Representor,
				MasterIndex: 1000,
			}}

			mocked.On("LinkByName", netconf.Representor).Return(fakeLink, nil)
			mocked.On("LinkSetDown", fakeLink).Return(nil)
			mocked.On("LinkSetNoMaster", fakeLink).Return(errors.New("some error"))

			m := manager{nLink: mocked}
			err := m.DetachRepresentor(netconf)
			Expect(err).To(HaveOccurred())
			mocked.AssertExpectations(t)
		})
	})
})
