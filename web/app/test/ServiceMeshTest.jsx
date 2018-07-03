import _ from 'lodash';
import Adapter from 'enzyme-adapter-react-16';
import podFixtures from './fixtures/podRollup.json';
import { expect } from 'chai';
import nsFixtures from './fixtures/namespaces.json';
import { routerWrap } from './testHelpers.jsx';
import ServiceMesh from '../js/components/ServiceMesh.jsx';
import sinon from 'sinon';
import sinonStubPromise from 'sinon-stub-promise';
import Enzyme, { mount } from 'enzyme';

Enzyme.configure({ adapter: new Adapter() });
sinonStubPromise(sinon);

describe('ServiceMesh', () => {
  let component, fetchStub;

  function withPromise(fn) {
    return component.find("ServiceMesh").instance().serverPromise.then(fn);
  }

  beforeEach(() => {
    fetchStub = sinon.stub(window, 'fetch').returnsPromise();
  });

  afterEach(() => {
    component = null;
    window.fetch.restore();
  });

  it("displays an error if the api call didn't go well", () => {
    let errorMsg = "Something went wrong!";

    fetchStub.resolves({
      ok: false,
      statusText: errorMsg
    });
    component = mount(routerWrap(ServiceMesh));

    return withPromise(() => {
      expect(component.html()).to.include(errorMsg);
    });
  });

  it("renders the spinner before metrics are loaded", () => {
    component = mount(routerWrap(ServiceMesh));

    expect(component.find(".ant-spin")).to.have.length(1);
    expect(component.find("ServiceMesh")).to.have.length(1);
    expect(component.find("CallToAction")).to.have.length(0);
  });

  it("renders a call to action if no metrics are received", () => {
    fetchStub.resolves({
      ok: true,
      json: () => Promise.resolve({ metrics: [] })
    });
    component = mount(routerWrap(ServiceMesh));

    return withPromise(() => {
      component.update();
      // console.log(component.find("Spin").debug());
      expect(component.find("ServiceMesh")).to.have.length(1);
      expect(component.find(".ant-spin")).to.have.length(0);
      expect(component.find("CallToAction")).to.have.length(1);
    });
  });

  it("renders controller component summaries", () => {
    fetchStub.resolves({
      ok: true,
      json: () => Promise.resolve(podFixtures)
    });
    component = mount(routerWrap(ServiceMesh));

    return withPromise(() => {
      component.update();
      expect(component.find("ServiceMesh")).to.have.length(1);
      expect(component.find(".ant-spin")).to.have.length(0);
    });
  });

  it("renders service mesh details section", () => {
    fetchStub.resolves({
      ok: true,
      json: () => Promise.resolve({ metrics: [] })
    });
    component = mount(routerWrap(ServiceMesh));

    return withPromise(() => {
      component.update();
      expect(component.find("ServiceMesh")).to.have.length(1);
      expect(component.find(".ant-spin")).to.have.length(0);
      expect(component.html()).to.include("Service mesh details");
      expect(component.html()).to.include("ShinyProductName version");
    });
  });

  it("renders control plane section", () => {
    fetchStub.resolves({
      ok: true,
      json: () => Promise.resolve({ metrics: [] })
    });
    component = mount(routerWrap(ServiceMesh));

    return withPromise(() => {
      component.update();
      expect(component.find("ServiceMesh")).to.have.length(1);
      expect(component.find(".ant-spin")).to.have.length(0);
      expect(component.html()).to.include("Control plane");
    });
  });

  it("renders data plane section", () => {
    fetchStub.resolves({
      ok: true,
      json: () => Promise.resolve({ metrics: [] })
    });
    component = mount(routerWrap(ServiceMesh));

    return withPromise(() => {
      component.update();
      expect(component.find("ServiceMesh")).to.have.length(1);
      expect(component.find(".ant-spin")).to.have.length(0);
      expect(component.html()).to.include("Data plane");
    });
  });

  describe("renderAddDeploymentsMessage", () => {
    it("displays when no resources are in the mesh", () => {
      fetchStub.resolves({
        ok: true,
        json: () => Promise.resolve({})
      });
      component = mount(routerWrap(ServiceMesh));

      return withPromise(() => {
        expect(component.html()).to.include("No resources detected");
      });
    });

    it("displays a message if >1 resource has not been added to the mesh", () => {
      let nsAllResourcesAdded = _.cloneDeep(nsFixtures);
      nsAllResourcesAdded.ok.statTables[0].podGroup.rows.push({
        "resource":{
          "namespace":"",
          "type":"namespaces",
          "name":"test-1"
        },
        "timeWindow": "1m",
        "meshedPodCount": "0",
        "runningPodCount": "5",
        "stats": null
      });

      fetchStub.resolves({
        ok: true,
        json: () => Promise.resolve(nsAllResourcesAdded)
      });
      component = mount(routerWrap(ServiceMesh));

      return withPromise(() => {
        expect(component.html()).to.include("4 namespaces have no meshed resources.");
      });
    });

    it("displays a message if 1 resource has not added to servicemesh", () => {
      let nsOneResourceNotAdded = _.cloneDeep(nsFixtures);
      _.each(nsOneResourceNotAdded.ok.statTables[0].podGroup.rows, row => {
        // set all namespaces to have fully meshed pod counts, except one
        if (row.resource.name !== "default") {
          row.meshedPodCount = "10";
          row.runningPodCount = "10";
        }
      });
      fetchStub.resolves({
        ok: true,
        json: () => Promise.resolve(nsOneResourceNotAdded)
      });
      component = mount(routerWrap(ServiceMesh));

      return withPromise(() => {
        expect(component.html()).to.include("1 namespace has no meshed resources.");
      });
    });

    it("displays a message if all resources have been added to servicemesh", () => {
      let nsAllResourcesAdded = _.cloneDeep(nsFixtures);
      _.each(nsAllResourcesAdded.ok.statTables[0].podGroup.rows, row => {
        row.meshedPodCount = "10";
        row.runningPodCount = "10";
      });
      fetchStub.resolves({
        ok: true,
        json: () => Promise.resolve(nsAllResourcesAdded)
      });
      component = mount(routerWrap(ServiceMesh));

      return withPromise(() => {
        expect(component.html()).to.include("All namespaces have a conduit install.");
      });
    });
  });
});
